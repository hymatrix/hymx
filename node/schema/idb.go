package schema

import (
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type IDB interface {
	// result
	SaveResult(result vmmSchema.VmmResult) error
	GetResult(msgid string) (result *vmmSchema.VmmResult, err error)
	GetResults(pid string, limit int64) (results []vmmSchema.VmmResult, err error)

	// assignment
	IsExist(pid string) (ok bool, err error)
	GetNonce(pid string) (nonce int64, err error)
	Commit(pid string, nonce int64, msg, assign goarSchema.BundleItem) (err error)

	// get
	GetAllProcess() (pids []string, curNonces []int64, err error)
	GetMessage(msgid string) (msg *goarSchema.BundleItem, err error)
	GetMessageByNonce(pid string, nonce int64) (msg *goarSchema.BundleItem, err error)
	GetAssignByNonce(pid string, nonce int64) (assign *goarSchema.BundleItem, err error)

	// checkpoint
	GetCheckpointIndex(pid string) (id string, err error)
	SaveCheckpointIndex(pid, id string) (err error)

	// cache
	GetCache(pid, key string) (value string, err error)
	SaveCache(pid, key, value string) (err error)
}

type IDBOutbox interface {
	Push(pid, target string, message goarSchema.BundleItem) error
	Peek(pid, target string) (*goarSchema.BundleItem, error)
	Commit(pid, target string, assign goarSchema.BundleItem) error

	Checkpoint(pid string) (string, error)
	Restore(data string) error
}
