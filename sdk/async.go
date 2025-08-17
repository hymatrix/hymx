package sdk

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/hymatrix/hymx/schema"
	serverSchema "github.com/hymatrix/hymx/server/schema"
	"github.com/hymatrix/hymx/utils"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (s *SDK) ResultAndWait(msgid string) (result vmmSchema.Result, err error) {
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return result, fmt.Errorf("timeout waiting for result after 2 minutes")
		case <-ticker.C:
			result, err = s.Client.GetResult(msgid)
			if err != nil {
				return result, err
			}
			if result.ItemId == "" {
				continue
			}
			return result, nil
		}
	}
}

func (s *SDK) SendAndWait(processId, data string, tags []goarSchema.Tag) (res *serverSchema.Response, err error) {
	var redirectUrl string
	res, redirectUrl, err = s.Send(processId, data, tags)
	if err != nil {
		return
	}

	// Handle redirect
	realSdk := s
	if redirectUrl != "" {
		log.Debug("redirect to url", "url", redirectUrl)
		realSdk = NewFromBundler(redirectUrl, s.Bundler)
	}

	result, err := realSdk.ResultAndWait(res.Id)
	if err != nil {
		return
	}
	jsonResult, err := json.Marshal(result)
	if err != nil {
		return
	}
	return &serverSchema.Response{
		Id:      res.Id,
		Message: string(jsonResult),
	}, nil
}

func (s *SDK) SendMessageAndWait(target, data string, params []goarSchema.Tag) (*serverSchema.Response, error) {
	msg := schema.Message{
		Base: schema.DefaultBaseMessage,
	}
	msgTags, err := utils.MessageToTags(msg)
	if err != nil {
		return nil, err
	}
	// merge params -> tags
	msgTags = utils.MergeTags(msgTags, params)
	return s.SendAndWait(target, data, msgTags)
}

func (s *SDK) SpawnAndWait(module, scheduler string, params []goarSchema.Tag) (*serverSchema.Response, error) {
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
	return s.SendAndWait("", strconv.Itoa(int(time.Now().UnixNano())), processTags)
}
