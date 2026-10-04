package vmm

import (
	"maps"
	"slices"

	"github.com/hymatrix/hymx/vmm/schema"
)

func cloneEnv(env schema.Env) schema.Env {
	env.Meta.Params = maps.Clone(env.Meta.Params)
	env.ReceivedSeq = maps.Clone(env.ReceivedSeq)
	env.Process.Tags = slices.Clone(env.Process.Tags)
	env.Module.Tags = slices.Clone(env.Module.Tags)
	return env
}

func (v *Vmm) instance(pid string, create bool) (*schema.VmInstance, error) {
	v.vmsLockMu.Lock()
	defer v.vmsLockMu.Unlock()
	if v.closing {
		return nil, schema.ErrVmmClosed
	}
	instance := v.vms[pid]
	if instance == nil && create {
		instance = &schema.VmInstance{
			Tasks:    make(chan *schema.VmTask, schema.VmQueueCapacity),
			Stopping: make(chan struct{}),
		}
		v.vms[pid] = instance
		v.wg.Add(1)
		go v.runInstance(pid, instance)
	}
	if instance == nil {
		return nil, schema.ErrProcessNotFound
	}
	return instance, nil
}

func (v *Vmm) submit(pid string, create bool, task *schema.VmTask) error {
	instance, err := v.instance(pid, create)
	if err != nil {
		return err
	}
	instance.Mu.Lock()
	if v.ctx.Err() != nil {
		instance.Mu.Unlock()
		return schema.ErrVmmClosed
	}
	if instance.StopTask != nil {
		instance.Mu.Unlock()
		return schema.ErrVmStopping
	}
	if !create && !instance.Loaded {
		instance.Mu.Unlock()
		return schema.ErrProcessNotFound
	}
	instance.Sending++
	instance.Senders.Add(1)
	task.Admitted = make(chan struct{})
	instance.Mu.Unlock()
	defer func() {
		instance.Mu.Lock()
		instance.Sending--
		instance.Mu.Unlock()
		instance.Senders.Done()
		close(task.Admitted)
	}()
	select {
	case instance.Tasks <- task:
		return nil
	case <-v.ctx.Done():
		return schema.ErrVmmClosed
	case <-instance.Stopping:
		if v.ctx.Err() != nil {
			return schema.ErrVmmClosed
		}
		return schema.ErrVmStopping
	}
}

func (v *Vmm) request(pid string, create bool, run func(*schema.VmInstance) error) error {
	task := &schema.VmTask{Run: run, Done: make(chan struct{})}
	if err := v.submit(pid, create, task); err != nil {
		return err
	}
	<-task.Done
	return task.Err
}

func publishVm(instance *schema.VmInstance, vm schema.Vm, env schema.Env) {
	instance.Vm, instance.Env = vm, cloneEnv(env)
	instance.Mu.Lock()
	instance.Loaded = true
	instance.Mu.Unlock()
}
