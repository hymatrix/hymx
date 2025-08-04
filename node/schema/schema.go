package schema

import (
	hymxSchema "github.com/hymatrix/hymx/schema"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
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

type AssignmentResult struct {
	Pid        string
	Item       goarSchema.BundleItem
	Assign     hymxSchema.Assignment
	AssignItem goarSchema.BundleItem
	Error      error
}

type AssignmentHandler func(AssignmentResult)
