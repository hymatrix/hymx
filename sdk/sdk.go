package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hymatrix/hymx/chainkit/optgoar"
	"github.com/hymatrix/hymx/common"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	serverSchema "github.com/hymatrix/hymx/server/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
)

var log = common.NewLog("sdk")

type SDK struct {
	Bundler *goar.Bundler
	Client  *Client
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
		Bundler: bundler,
		Client:  NewClient(nodeURL),
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

	item, err := s.Bundler.CreateAndSignItem([]byte(data), processId, "", tags)
	if err != nil {
		return
	}

	return s.Client.Send(item.Binary)
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

func (s *SDK) GenerateModule(moduleBytes []byte, module schema.Module) (item goarSchema.BundleItem, err error) {
	tags, err := utils.ModuleToTags(module)
	if err != nil {
		return
	}

	return s.Bundler.CreateAndSignItem(moduleBytes, "", "", tags)
}

func (s *SDK) GenerateAndSaveModule(moduleBytes []byte, module schema.Module) (itemId string, err error) {
	item, err := s.GenerateModule(moduleBytes, module)
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
	wallet, err := goar.NewWalletFromPath(keyfile, "https://arweave.net")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opt := optgoar.New(wallet, ctx)

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

	return opt.Upload([]goarSchema.BundleItem{item})
}
