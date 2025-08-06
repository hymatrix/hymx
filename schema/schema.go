package schema

import (
	goarSchema "github.com/permadao/goar/schema"
)

const (
	DataProtocol = "hymx"
	Variant      = "v0.1.1"

	TypeModule            = "Module"
	TypeProcess           = "Process"
	TypeMessage           = "Message"
	TypeAssignment        = "Assignment"
	TypeSchedulerLocation = "Scheduler-Location"
	TypeSchedulerTransfer = "Scheduler-Transfer"
	TypeCheckpoint        = "Checkpoint"
)

var (
	DefaultBaseModule = Base{
		DataProtocol: DataProtocol,
		Variant:      Variant,
		Type:         TypeModule,
	}

	DefaultBaseProcess = Base{
		DataProtocol: DataProtocol,
		Variant:      Variant,
		Type:         TypeProcess,
	}

	DefaultBaseMessage = Base{
		DataProtocol: DataProtocol,
		Variant:      Variant,
		Type:         TypeMessage,
	}

	DefaultBaseAssignment = Base{
		DataProtocol: DataProtocol,
		Variant:      Variant,
		Type:         TypeAssignment,
	}

	DefaultCheckpoint = Base{
		DataProtocol: DataProtocol,
		Variant:      Variant,
		Type:         TypeCheckpoint,
	}
)

type Base struct {
	DataProtocol string `json:"Data-Protocol"`
	Variant      string `json:"Variant"`
	Type         string `json:"Type"`
}

type Module struct {
	Base
	ModuleFormat   string           `json:"Module-Format"`
	MemoryLimit    string           `json:"Memory-Limit"`
	ComputeLimit   string           `json:"Compute-Limit"`
	InputEncoding  string           `json:"Input-Encoding,omitempty"`  // Json only now
	OutputEncoding string           `json:"Output-Encoding,omitempty"` // Json only now
	Tags           []goarSchema.Tag `json:"Tags,omitempty"`            // Extension
}

type Process struct {
	Base
	Module      string           `json:"Module"`
	Scheduler   string           `json:"Scheduler"`
	FromProcess string           `json:"From-Process,omitempty"`
	Tags        []goarSchema.Tag `json:"Tags"`
}

type Message struct {
	Base
	Action      string           `json:"Action"`
	FromProcess string           `json:"From-Process,omitempty"`
	PushedFor   string           `json:"Pushed-For"`
	Sequence    string           `json:"Sequence"`
	Tags        []goarSchema.Tag `json:"Tags"`
}

type Assignment struct {
	Base
	Process   string `json:"Process"`
	Message   string `json:"Message"`
	Nonce     string `json:"Nonce"`
	Timestamp string `json:"Timestamp"`
}

type Checkpoint struct {
	Base
	Process string `json:"Process"`
	Nonce   string `json:"Nonce"`
}
