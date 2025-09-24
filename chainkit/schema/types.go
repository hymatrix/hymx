package schema

import (
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type INode interface {
	// DB
	GetMessage(msgid string) (msg *goarSchema.BundleItem, err error)
	GetMessageByNonce(pid string, nonce int64) (msg *goarSchema.BundleItem, err error)

	GetAssignByMessage(msgid string) (assign *goarSchema.BundleItem, err error)
	GetAssignByNonce(pid string, nonce int64) (assign *goarSchema.BundleItem, err error)

	GetResult(msgid string) (result *vmmSchema.Result, err error)
}

type DownloadResult struct {
	Nonce      int64
	Assignment *goarSchema.BundleItem
	Message    *goarSchema.BundleItem
}
