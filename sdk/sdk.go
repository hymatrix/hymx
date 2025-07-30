package sdk

import (
	"fmt"
	"time"

	"github.com/hymatrix/hymx/common"
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

func (s *SDK) Send(processId, data string, tags []goarSchema.Tag) (res *serverSchema.Response, err error) {
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
func (s *SDK) SendMessage(target, data string, params []goarSchema.Tag) (*serverSchema.Response, error) {
	msg := schema.Message{
		Base: schema.DefaultBaseMessage,
	}
	msgTags, err := utils.MessageToTags(msg)
	if err != nil {
		return nil, err
	}
	// merge params -> tags
	msgTags = utils.MergeTags(msgTags, params)
	return s.Send(target, data, msgTags)
}

// Spawn for process creation
func (s *SDK) Spawn(module, scheduler string, params []goarSchema.Tag) (*serverSchema.Response, error) {
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
	return s.Send("", "", processTags)
}

func (s *SDK) GenerateModule(moduleBytes []byte, module schema.Module) (item goarSchema.BundleItem, err error) {
	tags, err := utils.ModuleToTags(module)
	if err != nil {
		return
	}

	return s.Bundler.CreateAndSignItem(moduleBytes, "", "", tags)
}
