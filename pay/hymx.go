package pay

import (
	"fmt"

	"github.com/hymatrix/hymx/utils"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (p *Pay) HymxHandler(res vmmSchema.Result) {
	if res.FromProcess != p.config.AxToken {
		return
	}

	tagsList := [][]goarSchema.Tag{}
	for _, msg := range res.Messages {
		if msg.Target == p.sdk.GetAddress() && utils.GetTagsValue("Action", msg.Tags) == "Credit-Notice" {
			tagsList = append(tagsList, msg.Tags)
		}
	}

	if len(tagsList) == 0 {
		return
	}

	for _, tags := range tagsList {
		fmt.Println(tags)
	}
}
