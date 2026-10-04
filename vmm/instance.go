package vmm

import (
	"maps"
	"slices"

	"github.com/hymatrix/hymx/vmm/schema"
)

func (v *Vmm) getInstance(pid string) (*schema.VmInstance, error) {
	v.vmsLockMu.RLock()
	defer v.vmsLockMu.RUnlock()
	if v.closed {
		return nil, schema.ErrVmmClosed
	}
	instance, ok := v.vms[pid]
	if !ok {
		return nil, schema.ErrProcessNotFound
	}
	return instance, nil
}

// createInstance reserves the PID and queues initialization before publishing it.
func (v *Vmm) createInstance(pid string, init func(*schema.VmInstance) error) (*schema.VmInstance, *schema.VmTask, error) {
	v.vmsLockMu.Lock()
	defer v.vmsLockMu.Unlock()
	if v.closed {
		return nil, nil, schema.ErrVmmClosed
	}
	if instance, ok := v.vms[pid]; ok {
		return instance, nil, schema.ErrProcessAlreadyExists
	}
	instance := &schema.VmInstance{
		Pid:   pid,
		Tasks: make(chan *schema.VmTask, schema.VmQueueCapacity),
		Done:  make(chan struct{}),
	}
	task := &schema.VmTask{Done: make(chan struct{}), Run: func(instance *schema.VmInstance) error {
		instance.InitErr = init(instance)
		if instance.InitErr != nil {
			v.removeInstance(instance)
			v.stopInstance(instance)
		} else {
			v.coreMu.RLock()
			isRegistry := v.registry != nil && instance.Vm == v.registry
			v.coreMu.RUnlock()
			if isRegistry && v.registrySpawned != nil {
				v.registryReady.Do(func() { close(v.registrySpawned) })
			}
		}
		return instance.InitErr
	}}
	instance.Tasks <- task
	v.vms[pid] = instance
	v.wg.Add(1)
	go v.runInstance(instance)
	return instance, task, nil
}

func (v *Vmm) submit(instance *schema.VmInstance, task *schema.VmTask) error {
	// Admission and Close are ordered by the map lock, but sending never holds it.
	v.vmsLockMu.RLock()
	if v.closed {
		v.vmsLockMu.RUnlock()
		return schema.ErrVmmClosed
	}
	instance.Mu.Lock()
	if instance.Stopping {
		instance.Mu.Unlock()
		v.vmsLockMu.RUnlock()
		return schema.ErrVmStopping
	}
	instance.Submitters.Add(1)
	instance.Mu.Unlock()
	v.vmsLockMu.RUnlock()
	defer instance.Submitters.Done()
	instance.Tasks <- task
	return nil
}

func (v *Vmm) call(instance *schema.VmInstance, run func(*schema.VmInstance) error) error {
	task := &schema.VmTask{Run: run, Done: make(chan struct{})}
	if err := v.submit(instance, task); err != nil {
		return err
	}
	<-task.Done
	return task.Err
}

func (v *Vmm) stopInstance(instance *schema.VmInstance) {
	instance.StopOnce.Do(func() {
		instance.Mu.Lock()
		instance.Stopping = true
		instance.Mu.Unlock()
		go func() {
			instance.Submitters.Wait()
			close(instance.Tasks)
		}()
	})
}

func (v *Vmm) removeInstance(instance *schema.VmInstance) {
	v.vmsLockMu.Lock()
	defer v.vmsLockMu.Unlock()
	if v.vms[instance.Pid] == instance {
		delete(v.vms, instance.Pid)
	}
}

func (v *Vmm) runInstance(instance *schema.VmInstance) {
	defer v.wg.Done()
	defer close(instance.Done)
	for task := range instance.Tasks {
		task.Err = instance.InitErr
		if task.Err == nil {
			task.Err = task.Run(instance)
		}
		if task.Done != nil {
			close(task.Done)
		} else if task.Err != nil {
			log.Error("apply failed", "pid", instance.Pid, "err", task.Err)
		}
	}
	if instance.Vm != nil {
		instance.CloseErr = instance.Vm.Close()
	}
	if instance.InitErr != nil {
		v.coreMu.Lock()
		if v.registry != nil && instance.Vm == v.registry {
			v.registry = nil
		}
		if v.token != nil && instance.Vm == v.token {
			v.token = nil
		}
		v.coreMu.Unlock()
	}
	// The task loop has stopped even when the VM fails to release its resources.
	v.removeInstance(instance)
	if instance.CloseErr != nil {
		log.Error("kill process failed", "pid", instance.Pid, "err", instance.CloseErr)
	}
}

func cloneMeta(meta schema.Meta) schema.Meta {
	meta.Params = maps.Clone(meta.Params)
	return meta
}

func cloneEnv(env schema.Env) schema.Env {
	env.Meta = cloneMeta(env.Meta)
	env.Module.Tags = slices.Clone(env.Module.Tags)
	env.Process.Tags = slices.Clone(env.Process.Tags)
	env.ReceivedSeq = maps.Clone(env.ReceivedSeq)
	if env.ReceivedSeq == nil {
		env.ReceivedSeq = map[string]int64{}
	}
	return env
}
