package schema

import (
	"time"

	"github.com/hymatrix/hymx/vmm/core/registry/schema"
)

const (
	NodeVersion  = "v0.4.1"
	GenesisAccId = "0x18b4bA4c118279b3eB60a2DB1E794Bc41AFC1D37"
)

var (
	GenesisNode = schema.Node{
		AccId: GenesisAccId,
		Name:  "TestGenesisNode",
		Role:  schema.RoleMain,
		Desc:  "Test network genesis node",
		URL:   "https://hymatrix.ai",
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
