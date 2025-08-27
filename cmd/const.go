package main

import (
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
)

const (
	Pid = schema.DataProtocol + "-" + nodeSchema.NodeVersion + ".lock"

	DefaultConfig = "./config.yaml"
)
