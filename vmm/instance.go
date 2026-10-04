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
		instance = &schema.VmInstance{Wake: make(chan struct{}, schema.VmWakeCapacity)}
		v.vms[pid] = instance
		v.wg.Add(1)
		go v.runInstance(pid, instance)
	}
	if instance == nil {
		return nil, schema.ErrProcessNotFound
	}
	return instance, nil
}

// queueTask requires Mu. Queue growth never waits for the executing VM.
func queueTask(instance *schema.VmInstance, task *schema.VmTask) {
	instance.Tasks = append(instance.Tasks, task)
	select {
	case instance.Wake <- struct{}{}:
	default:
	}
}

func (v *Vmm) submit(pid string, create bool, task *schema.VmTask) error {
	instance, err := v.instance(pid, create)
	if err != nil {
		return err
	}
	instance.Mu.Lock()
	defer instance.Mu.Unlock()
	if instance.StopTask != nil {
		return schema.ErrVmStopping
	}
	if !create && !instance.Loaded {
		return schema.ErrProcessNotFound
	}
	queueTask(instance, task)
	return nil
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
