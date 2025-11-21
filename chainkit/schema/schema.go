package schema

import (
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type Config struct {
	RedisUrl     string `json:"redisUrl" yaml:"redisUrl"`
	NodeRedisUrl string `json:"nodeRedisUrl" yaml:"nodeRedisUrl"`
	Keyfile      string `json:"keyfile" yaml:"keyfile"`
	OptType      string `json:"optType" yaml:"optType"`
}

type DownloadResult struct {
	Nonce      int64
	Assignment *goarSchema.BundleItem
	Message    *goarSchema.BundleItem
}

type IDBTool interface {
	// DB
	GetMessage(msgid string) (msg *goarSchema.BundleItem, err error)
	GetAssignByNonce(pid string, nonce int64) (assign *goarSchema.BundleItem, err error)
	GetResult(msgid string) (result *vmmSchema.VmmResult, err error)
}

type IDBChainkit interface {
	AddPending(txid string) error
	MoveToUploading() (int64, error)
	EndUpload() error

	IsUploadedBatch(txids []string) (map[string]bool, error)
	IsUploaded(txid string) (bool, error)

	GetUploading() ([]string, error)
	SetBundledIn(bundledInID string) error
	GetBundledIn() (string, error)

	Cache(txid string, item goarSchema.BundleItem) error
	GetCache(txid string) (*goarSchema.BundleItem, error)
}
