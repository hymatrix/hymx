package schema

import "github.com/hymatrix/hymx/vmm/core/registry/schema"

const NodeVersion = "v0.1.3"

var (
	GenesisAccId = "0x18b4bA4c118279b3eB60a2DB1E794Bc41AFC1D37"
	// GenesisAccId = "0x972AeD684D6f817e1b58AF70933dF1b4a75bfA51"
	GenesisNode = schema.Node{
		AccId: GenesisAccId,
		Name:  "TestGenesisNode",
		Role:  schema.RoleMain,
		Desc:  "Test network genesis node",
		URL:   "https://hymatrix.ai",
		// URL:   "http://127.0.0.1:8080",
	}

	NonExtractableTags = map[string]string{
		"Data-Protocol": "Data-Protocol",
		"Variant":       "Variant",
		"From-Process":  "From-Process",
		"From-Module":   "From-Module",
		"Type":          "Type",
		"From":          "From",
		"Owner":         "Owner",
		"Anchor":        "Anchor",
		"Target":        "Target",
		"Data":          "Data",
		"Tags":          "Tags",
		"Read-Only":     "Read-Only",
	}
)
