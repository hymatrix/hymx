package schema

import (
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

type Info struct {
	Protocol    string              `json:"Protocol"`
	Variant     string              `json:"Variant"`
	JoinNetwork bool                `json:"Join-Network"`
	Token       string              `json:"Token"`
	Registry    string              `json:"Registry"`
	Node        registrySchema.Node `json:"Node"`
}

type ResultHandler func(vmmSchema.Result)
