package schema

import (
	"math/big"

	"github.com/hymatrix/hymx/vmm/core/registry/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type TokenSnapshot struct {
	Id        string `json:"i"`
	Name      string `json:"n"`
	Ticker    string `json:"t"`
	Decimals  int64  `json:"d"`
	Logo      string `json:"l"`
	MinAmount *big.Int

	TotalSupply *big.Int            `json:"ts"`
	Balances    map[string]*big.Int `json:"bals"`
	Stakes      map[string]*big.Int `json:"stks"`
}

type RegistrySnapshot struct {
	Id        string `json:"i"`
	TokenPid  string `json:"tp"`
	MainIndex string `json:"mi"`

	ProcessToNodeIndex    map[string]map[string]*schema.Node `json:"pi"`
	AccidToProcessesIndex map[string]map[string]string       `json:"ai"`
	Registered            map[string]bool                    `json:"re"`
	Nodes                 map[string]*schema.Node            `json:"n"`
}

type OutboxSnapshot struct {
	Id      string                   `json:"i"`
	Mailbox []*goarSchema.BundleItem `json:"m"`
	Targets map[string][]int         `json:"t"`
}

type PaySnapshot struct {
	Whitelist        map[string]bool                `json:"w"`
	Executed         map[string]bool                `json:"e"`
	Ledger           map[string]map[string]*big.Int `json:"l"`
	TxPending        map[string]map[string]*big.Int `json:"tp"`
	SpawnPending     map[string]map[string]*big.Int `json:"sp"`
	ResidencyPending map[string]*big.Int            `json:"rp"`
	DailyUsage       map[string]int64               `json:"du"`
}
