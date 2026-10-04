package vmm

import "github.com/hymatrix/hymx/vmm/schema"

func (v *Vmm) Mount(moduleFormat string, spawner schema.VmSpawnFunc) error {
	v.vmsLockMu.Lock()
	defer v.vmsLockMu.Unlock()

	if _, ok := v.vmFactors[moduleFormat]; ok {
		return schema.ErrFactoryAlreadyMounted
	}

	v.vmFactors[moduleFormat] = spawner
	return nil
}

func (v *Vmm) stop(pid string, instance *schema.VmInstance) *schema.VmTask {
	instance.Mu.Lock()
	defer instance.Mu.Unlock()
	if instance.StopTask != nil {
		return instance.StopTask
	}
	task := &schema.VmTask{Done: make(chan struct{}), Stop: true}
	task.Run = func(instance *schema.VmInstance) error {
		if instance.Vm != nil {
			if err := instance.Vm.Close(); err != nil {
				return err
			}
		}
		v.vmsLockMu.Lock()
		if v.vms[pid] == instance {
			delete(v.vms, pid)
			delete(v.vmsRecoveryLock, pid)
		}
		v.vmsLockMu.Unlock()
		return nil
	}
	instance.StopTask = task
	// Stop admission without needing a free slot in the task channel.
	close(instance.Stopping)
	return task
}

func (v *Vmm) Kill(pid string) error {
	instance, err := v.instance(pid, false)
	if err != nil {
		return err
	}
	task := v.stop(pid, instance)
	<-task.Done
	return task.Err
}

func (v *Vmm) KillAll() {
	v.vmsLockMu.RLock()
	tasks := make(map[string]*schema.VmTask, len(v.vms))
	for pid, instance := range v.vms {
		tasks[pid] = v.stop(pid, instance)
	}
	v.vmsLockMu.RUnlock()
	for pid, task := range tasks {
		<-task.Done
		if task.Err != nil {
			log.Error("kill process failed", "pid", pid, "err", task.Err)
		}
	}
}

func (v *Vmm) IsExists(pid string) bool {
	v.vmsLockMu.RLock()
	instance := v.vms[pid]
	v.vmsLockMu.RUnlock()
	if instance == nil {
		return false
	}
	instance.Mu.Lock()
	defer instance.Mu.Unlock()
	return instance.Loaded
}

// GetVm preserves the legacy interface and returns an independent Env.
// Direct calls on the returned VM bypass VMM serialization.
func (v *Vmm) GetVm(pid string) (vm schema.Vm, env *schema.Env, err error) {
	err = v.request(pid, false, func(instance *schema.VmInstance) error {
		copy := cloneEnv(instance.Env)
		vm, env = instance.Vm, &copy
		return nil
	})
	return
}

func (v *Vmm) GetVmPids() (pids []string) {
	v.vmsLockMu.RLock()
	defer v.vmsLockMu.RUnlock()
	for pid, instance := range v.vms {
		instance.Mu.Lock()
		if instance.Loaded {
			pids = append(pids, pid)
		}
		instance.Mu.Unlock()
	}
	return
}

func (v *Vmm) GetModuleNames() (names []string) {
	v.vmsLockMu.RLock()
	defer v.vmsLockMu.RUnlock()

	names = make([]string, 0, len(v.vmFactors))
	for name := range v.vmFactors {
		names = append(names, name)
	}
	return
}

func (v *Vmm) GetVmCount() int64 { return int64(len(v.GetVmPids())) }

func (v *Vmm) RecoveryLock(pid string) {
	v.vmsLockMu.Lock()
	defer v.vmsLockMu.Unlock()

	v.vmsRecoveryLock[pid] = true
}

func (v *Vmm) RecoveryUnlock(pid string) {
	v.vmsLockMu.Lock()
	defer v.vmsLockMu.Unlock()

	delete(v.vmsRecoveryLock, pid)
}

func (v *Vmm) IsRecovering(pid string) bool {
	v.vmsLockMu.RLock()
	defer v.vmsLockMu.RUnlock()

	return v.vmsRecoveryLock[pid]
}

func (v *Vmm) Checkpoint(pid string) (snap schema.Snapshot, err error) {
	err = v.request(pid, false, func(instance *schema.VmInstance) error {
		snap.Data, snap.Err = instance.Vm.Checkpoint()
		snap.Env = cloneEnv(instance.Env)
		return snap.Err
	})
	return
}

func (v *Vmm) Restore(snap schema.Snapshot) error {
	snap.Env = cloneEnv(snap.Env)
	return v.request(snap.Env.Meta.Pid, true, func(instance *schema.VmInstance) error {
		vm := instance.Vm
		if vm == nil {
			var err error
			vm, err = v.spawn(cloneEnv(snap.Env))
			if err != nil {
				return err
			}
		}
		if err := vm.Restore(snap.Data); err != nil {
			if instance.Vm == nil {
				_ = vm.Close()
			}
			return err
		}
		publishVm(instance, vm, snap.Env)
		return nil
	})
}
