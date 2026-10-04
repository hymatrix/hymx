package schema

import "sync"

const VmWakeCapacity = 1

// Vm and Env belong to the worker. Mu protects queue admission and Loaded.
type VmInstance struct {
	Vm       Vm
	Env      Env
	Mu       sync.Mutex
	Tasks    []*VmTask
	Wake     chan struct{}
	Loaded   bool
	StopTask *VmTask
}

// Run executes in the PID worker; Err is readable after Done closes.
type VmTask struct {
	Run  func(*VmInstance) error
	Done chan struct{}
	Err  error
	Stop bool
}
