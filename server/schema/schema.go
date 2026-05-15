package schema

import (
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

type Response struct {
	Id      string `json:"id"`
	Message string `json:"message"`
}

type ResponseResults struct {
	Edges []ResultsEdge `json:"edges"`
}

type ResultsEdge struct {
	Cursor string              `json:"cursor"`
	Node   vmmSchema.VmmResult `json:"node"`
}

type Cursor struct {
	Timestamp int64  `json:"timestamp"`
	Ordinate  int64  `json:"ordinate"`
	Cron      string `json:"cron"`
	Sort      string `json:"sort"`
}

type TrySendRequest struct {
	Pid    string `json:"pid"`
	Target string `json:"target"`
}

type RequestVM struct {
	Pid string `json:"pid"`
}

type ResponseRunningVMs struct {
	Pids []string `json:"pids"`
}
