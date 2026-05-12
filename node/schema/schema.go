package schema

import (
	hymxSchema "github.com/hymatrix/hymx/schema"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

const (
	StartModeNormal  = "normal"
	StartModeRebuild = "rebuild"
)

type Info struct {
	Protocol            string              `json:"Protocol"`
	Variant             string              `json:"Variant"`
	NodeVersion         string              `json:"Node-Version"`
	JoinNetwork         bool                `json:"Join-Network"`
	Token               string              `json:"Token"`
	Registry            string              `json:"Registry"`
	Node                registrySchema.Node `json:"Node"`
	VmCount             int64               `json:"Vm-Count"`
	EncryptionPublicKey string              `json:"Encryption-Public-Key"`
	EncryptionKeyType   string              `json:"Encryption-Key-Type"`
}

type ItemMeta struct {
	Pid         string
	Signer      string
	FromProcess string
	Instance    interface{}
}

type ItemHandler func(ItemMeta) error

type AssignmentResult struct {
	Pid        string
	Item       goarSchema.BundleItem
	Assign     hymxSchema.Assignment
	AssignItem goarSchema.BundleItem
	Error      error
}

type AssignResHandler func(AssignmentResult)

type ResultHandler func(vmmSchema.VmmResult)
