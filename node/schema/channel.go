package schema

import (
	"github.com/hymatrix/hymx/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type AssignMessage struct {
	Pid     string
	AccId   string
	Message schema.Message
	Item    goarSchema.BundleItem
}

type AssignProcess struct {
	Pid     string
	AccId   string
	Process schema.Process
	Item    goarSchema.BundleItem
}
