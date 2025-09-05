package schema

import (
	"github.com/hymatrix/hymx/schema"
	goarSchema "github.com/permadao/goar/schema"
)

const (
	ModuleFormatToken    = "hymx.core.token.0.0.0"
	ModuleFormatRegistry = "hymx.core.registry.0.0.0"

	AccountTypeAR  = "arweave"
	AccountTypeEVM = "evm"
)

type VmSpawnFunc func(Env) (Vm, error)

type Vm interface {
	Apply(from string, meta Meta) (res *Result, err error)
	Checkpoint() (data string, err error)
	Restore(data string) error
	Close() error
}

type Meta struct {
	// from item
	ItemId string `json:"Item-Id"`
	Pid    string `json:"Pid"`
	AccId  string `json:"Acc-Id"`
	// from message
	Action      string `json:"Action"`
	FromProcess string `json:"From-Process"`
	PushedFor   string `json:"Pushed-For"`
	Sequence    int64  `json:"Sequence"`
	// from assignment
	Nonce     int64 `json:"Nonce"`
	Timestamp int64 `json:"Timestamp"`
	// input params
	Params map[string]string `json:"Params"`
	Data   string            `json:"Data"`

	DryRun           bool  `json:"-"`
	RecoveryMaxNonce int64 `json:"-"`
}

type Result struct {
	Nonce        string            `json:"Nonce"`
	Timestamp    string            `json:"Timestamp"`
	ItemId       string            `json:"Item-Id"`
	FromProcess  string            `json:"From-Process"` // FromProcess is the source process (Pid) that produced this Result
	PushedFor    string            `json:"Pushed-For"`
	Messages     []*ResMessage     `json:"Messages"`
	Spawns       []*ResSpawn       `json:"Spawns"`
	Assignmengts []interface{}     `json:"Assignmengts"`
	Output       interface{}       `json:"Output"`
	Data         string            `json:"Data"`
	Cache        map[string]string `json:"Cache,omitempty"` // Cache contains the generated cache entries for users to read and query latest state
	DryRun       bool              `json:"-"`
	Error        string            `json:"Error"`
}

type ResMessage struct {
	Sequence string           `json:"Sequence"`
	Target   string           `json:"Target"`
	Data     string           `json:"Data,omitempty"`
	Tags     []goarSchema.Tag `json:"Tags"`
}

type ResSpawn struct {
	Sequance string           `json:"Sequance"`
	Data     string           `json:"Data,omitempty"`
	Tags     []goarSchema.Tag `json:"Tags"`
}

type Env struct {
	Meta Meta `json:"Meta"`

	Id    string `json:"Id"`
	AccId string `json:"Acc-Id"`

	Process schema.Process `json:"Process"`
	Module  schema.Module  `json:"Module"`

	Nonce    int64 `json:"Nonce"`    // inbox nonce
	Sequence int64 `json:"Sequence"` // outbox sequence

	ReceivedSeq map[string]int64 `json:"Received-Sequence"` // Received msg from other address/process, addr -> sequence number
}

type Outbox struct {
	Type string
	To   string
	From string
	Data string
	Tags []goarSchema.Tag
}

type Snapshot struct {
	Env    Env    `json:"Env"`
	Data   string `json:"Data"`
	Outbox string `json:"Outbox"`
	Err    error  `json:"-"`
}

type Checkpoint struct {
	Pid string
	Res chan Snapshot
}
