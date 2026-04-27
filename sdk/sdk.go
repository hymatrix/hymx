package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/hymatrix/hymx/chainkit/optgoar"
	"github.com/hymatrix/hymx/common"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	serverSchema "github.com/hymatrix/hymx/server/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/utils/tagcrypto"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
)

var log = common.NewLog("sdk")

type SDK struct {
	Bundler   *goar.Bundler
	Client    *Client
	infoMu    sync.Mutex
	infoByURL map[string]nodeSchema.Info
}

func New(nodeURL, keyPath string) *SDK {
	signer, err := goar.NewSignerFromPath(keyPath)
	if err != nil {
		panic(err)
	}
	bundler, err := goar.NewBundler(signer)
	if err != nil {
		panic(err)
	}

	return NewFromBundler(nodeURL, bundler)
}

func NewFromBundler(nodeURL string, bundler *goar.Bundler) *SDK {
	defer log.Info("wallet initialized", "wallet", bundler.Address)

	return &SDK{
		Bundler:   bundler,
		Client:    NewClient(nodeURL),
		infoByURL: map[string]nodeSchema.Info{},
	}
}

func (s *SDK) Close() {
	s.Client.Close()
}

func (s *SDK) GetAddress() string {
	return s.Bundler.Address
}

func (s *SDK) Send(processId, data string, tags []goarSchema.Tag) (res *serverSchema.Response, redirectedURL string, err error) {
	tags = utils.MergeTags([]goarSchema.Tag{
		{Name: "SDK-Timestamp", Value: fmt.Sprintf("%d", time.Now().UnixNano())},
	}, tags)

	hasEncryptedTags := tagcrypto.HasEncryptedTags(tags)
	if err = tagcrypto.ValidateEncryptedTagNames(tags); err != nil {
		return
	}

	item, err := s.createSignedItem(s.Client.baseURL, processId, data, tags)
	if err != nil {
		return
	}
	if !hasEncryptedTags {
		return s.Client.Send(item.Binary)
	}

	res, nodes, err := s.postSignedItem(s.Client.baseURL, item.Binary)
	if err != nil || len(nodes) == 0 {
		return res, "", err
	}

	const maxEncryptedRedirects = 10
	queue := append([]registrySchema.Node(nil), nodes...)
	visited := map[string]bool{}
	for attempts := 0; len(queue) != 0 && attempts < maxEncryptedRedirects; {
		node := queue[0]
		queue = queue[1:]
		if node.URL == "" {
			continue
		}
		if visited[node.URL] {
			continue
		}
		visited[node.URL] = true
		attempts++
		redirectItem, itemErr := s.createSignedItem(node.URL, processId, data, tags)
		if itemErr != nil {
			log.Error("create redirected item failed", "url", node.URL, "err", itemErr)
			continue
		}
		res, nextNodes, itemErr := s.postSignedItem(node.URL, redirectItem.Binary)
		if itemErr != nil {
			log.Error("send redirected item failed", "url", node.URL, "err", itemErr)
			continue
		}
		if len(nextNodes) != 0 {
			queue = append(queue, nextNodes...)
			continue
		}
		return res, node.URL, nil
	}

	return nil, "", fmt.Errorf("invalid server response: %d", http.StatusPermanentRedirect)
}

func (s *SDK) createSignedItem(baseURL, processId, data string, tags []goarSchema.Tag) (goarSchema.BundleItem, error) {
	signTags := tags
	if tagcrypto.HasEncryptedTags(signTags) {
		info, err := s.info(baseURL)
		if err != nil {
			return goarSchema.BundleItem{}, err
		}
		if info.EncryptionPublicKey == "" || info.EncryptionKeyType == "" {
			return goarSchema.BundleItem{}, fmt.Errorf("node %s does not advertise encryption metadata", baseURL)
		}
		signTags, _, err = tagcrypto.EncryptTags(signTags, info.EncryptionPublicKey, info.EncryptionKeyType)
		if err != nil {
			return goarSchema.BundleItem{}, err
		}
	}
	return s.Bundler.CreateAndSignItem([]byte(data), processId, "", signTags)
}

func (s *SDK) info(baseURL string) (nodeSchema.Info, error) {
	s.infoMu.Lock()
	if info, ok := s.infoByURL[baseURL]; ok {
		s.infoMu.Unlock()
		return info, nil
	}
	s.infoMu.Unlock()

	infoURL, err := resolveURL(baseURL, "/info")
	if err != nil {
		return nodeSchema.Info{}, err
	}
	resp, err := s.Client.httpClient.Get(infoURL)
	if err != nil {
		return nodeSchema.Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nodeSchema.Info{}, fmt.Errorf("invalid server response: %d", resp.StatusCode)
	}

	var info nodeSchema.Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nodeSchema.Info{}, err
	}
	s.infoMu.Lock()
	if s.infoByURL == nil {
		s.infoByURL = map[string]nodeSchema.Info{}
	}
	s.infoByURL[baseURL] = info
	s.infoMu.Unlock()
	return info, nil
}

func (s *SDK) postSignedItem(baseURL string, itemBin []byte) (*serverSchema.Response, []registrySchema.Node, error) {
	endpointURL, err := resolveURL(baseURL, "/")
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequest("POST", endpointURL, bytes.NewBuffer(itemBin))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.Client.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPermanentRedirect {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read redirect response: %w", err)
		}
		nodes, err := parseRedirectNodes(body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse redirect response: %w", err)
		}
		if len(nodes) == 0 {
			return nil, nil, fmt.Errorf("redirect response has no usable nodes")
		}
		return nil, nodes, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("invalid server response: %d", resp.StatusCode)
	}

	res := &serverSchema.Response{}
	err = json.NewDecoder(resp.Body).Decode(res)
	return res, nil, err
}

func parseRedirectNodes(body []byte) ([]registrySchema.Node, error) {
	var nodes []registrySchema.Node
	if err := json.Unmarshal(body, &nodes); err == nil {
		return nodes, nil
	}

	var node registrySchema.Node
	if err := json.Unmarshal(body, &node); err != nil {
		return nil, err
	}
	if node.URL == "" {
		return nil, nil
	}
	return []registrySchema.Node{node}, nil
}

func resolveURL(base, path string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	relativePath, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(relativePath).String(), nil
}

// Send message to process
func (s *SDK) SendMessage(target, data string, params []goarSchema.Tag) (resp *serverSchema.Response, err error) {
	msg := schema.Message{
		Base: schema.DefaultBaseMessage,
	}
	msgTags, err := utils.MessageToTags(msg)
	if err != nil {
		return nil, err
	}
	// merge params -> tags
	msgTags = utils.MergeTags(msgTags, params)
	resp, _, err = s.Send(target, data, msgTags)
	return
}

// Spawn for process creation
func (s *SDK) Spawn(module, scheduler string, params []goarSchema.Tag) (resp *serverSchema.Response, err error) {
	process := schema.Process{
		Base:      schema.DefaultBaseProcess,
		Module:    module,
		Scheduler: scheduler,
	}
	processTags, err := utils.ProcessToTags(process)
	if err != nil {
		return nil, err
	}

	// merge params -> processTags
	processTags = utils.MergeTags(processTags, params)
	resp, _, err = s.Send("", "", processTags)
	return
}

func (s *SDK) GenModule(moduleBytes []byte, module schema.Module) (item goarSchema.BundleItem, err error) {
	tags, err := utils.ModuleToTags(module)
	if err != nil {
		return
	}

	return s.Bundler.CreateAndSignItem(moduleBytes, "", "", tags)
}

func (s *SDK) SaveModule(moduleBytes []byte, module schema.Module) (itemId string, err error) {
	item, err := s.GenModule(moduleBytes, module)
	if err != nil {
		return "", err
	}

	itemBin, err := json.Marshal(item)
	if err != nil {
		return "", err
	}
	filename := fmt.Sprintf("mod-%s.json", item.Id)
	err = os.WriteFile(filename, itemBin, 0644)
	return item.Id, err
}

func (s *SDK) UploadModuleToArweave(filePath, keyfile string) (txid string, err error) {
	_, err = os.Stat(filePath)
	if os.IsNotExist(err) {
		err = nodeSchema.ErrNotFoundMod
		return "", err
	} else if err != nil {
		return "", err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	var item goarSchema.BundleItem
	if err = json.Unmarshal(data, &item); err != nil {
		return "", err
	}

	wallet, err := goar.NewWalletFromPath(keyfile, "https://arweave.net")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opt := optgoar.New(wallet, ctx)

	return opt.Upload([]goarSchema.BundleItem{item})
}

func (s *SDK) DownloadModuleFromArweave(itemId string) (item *goarSchema.BundleItem, err error) {
	return optgoar.Download(itemId, goar.NewClient("https://arweave.net"))
}
