package schema

import (
	"time"

	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

var (
	ZeroParentID = "0"
)

// AggregationPolicy controls how aggregator groups transactions.
type AggregationPolicy struct {
	MaxItems int64         // flush when reaching item count
	MaxDelay time.Duration // flush when oldest item waits longer than this
}

type INode interface {
	// verify
	AuthNode(accid, fromProcess string) error
	VerifyFromProcess(item goarSchema.BundleItem, pid, signer, fromProcess string) error
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
