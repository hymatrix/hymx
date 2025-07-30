package utils

import (
	"github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/hymatrix/hymx/vmm/utils"
)

func Decode(params map[string]string) (node schema.Node, err error) {
	accid, ok := params["Acc-Id"]
	if !ok {
		err = schema.ErrMissingParam
		return
	}
	_, accid, err = utils.IDCheck(accid)
	if err != nil {
		return
	}

	name, ok := params["Name"]
	if !ok {
		err = schema.ErrMissingParam
		return
	}
	desc, ok := params["Desc"]
	if !ok {
		err = schema.ErrMissingParam
		return
	}
	url, ok := params["URL"]
	if !ok {
		err = schema.ErrMissingParam
		return
	}

	node = schema.Node{
		AccId: accid,
		Name:  name,
		Role:  schema.RoleFollower, // default: follower
		Desc:  desc,
		URL:   url,
	}
	return
}
