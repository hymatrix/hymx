package schema

import (
	goarSchema "github.com/permadao/goar/schema"
)

type INode interface {
	// DB
	GetMessage(msgid string) (msg *goarSchema.BundleItem, err error)
	GetAssignByMessage(msgid string) (assign *goarSchema.BundleItem, err error)
	GetAssignByNonce(pid string, nonce int64) (assign *goarSchema.BundleItem, err error)
}

type IDB interface {
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

type DownloadResult struct {
	Nonce      int64
	Assignment *goarSchema.BundleItem
	Message    *goarSchema.BundleItem
}
