package schema

import "sync"

const VmQueueCapacity = 64

// VmInstance owns one serial task loop. Pid and channel identities are immutable.
type VmInstance struct {
	Pid string
	// Vm, Env and InitErr belong to the task loop.
	Vm      Vm
	Env     *Env
	InitErr error
	Tasks   chan *VmTask

	// Mu gates admission; it is never held while sending or executing a task.
	Mu         sync.Mutex
	Stopping   bool
	Submitters sync.WaitGroup
	StopOnce   sync.Once
	Done       chan struct{}
	// CloseErr is readable after Done closes.
	CloseErr error
}

type VmTask struct {
	Run  func(*VmInstance) error
	Done chan struct{}
	Err  error
}
