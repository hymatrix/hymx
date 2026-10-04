package schema

import "sync"

const VmQueueCapacity = 64

// Vm and Env belong to the worker. Mu protects queue admission and Loaded.
type VmInstance struct {
	Vm       Vm
	Env      Env
	Mu       sync.Mutex
	Tasks    chan *VmTask
	Stopping chan struct{}
	Loaded   bool
	StopTask *VmTask
	// Sending is protected by Mu. Senders lets the worker finish admission before stopping.
	Sending int
	Senders sync.WaitGroup
}

// Run executes in the PID worker; Err is readable after Done closes.
type VmTask struct {
	Admitted chan struct{} // Closed after the submitting caller finishes admission.
	Run      func(*VmInstance) error
	Done     chan struct{}
	Err      error
	Stop     bool
}
