package schema

import (
	"time"

	"github.com/hymatrix/hymx/vmm/core/registry/schema"
)

const (
	NodeVersion  = "v0.3.0"
	GenesisAccId = "0x18b4bA4c118279b3eB60a2DB1E794Bc41AFC1D37"
	// GenesisAccId = "0x972AeD684D6f817e1b58AF70933dF1b4a75bfA51"
)

var (
	GenesisNode = schema.Node{
		AccId: GenesisAccId,
		Name:  "TestGenesisNode",
		Role:  schema.RoleMain,
		Desc:  "Test network genesis node",
		URL:   "https://hymatrix.ai",
		// URL: "http://127.0.0.1:8080",
	}

	TryAssignSleepTime = []time.Duration{
		0,
		500 * time.Millisecond,
		1 * time.Second,
		3 * time.Second,
		5 * time.Second,
		10 * time.Second,
		20 * time.Second,
		50 * time.Second,
		80 * time.Second,
	}
)
