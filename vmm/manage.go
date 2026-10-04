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

func (v *Vmm) Kill(pid string) (err error) {
	instance, err := v.getInstance(pid)
	if err != nil {
		return
	}

	v.stopInstance(instance)
	<-instance.Done
	return instance.CloseErr
}

func (v *Vmm) KillAll() {
	pids := v.GetVmPids()
	if len(pids) == 0 {
		return
	}

	for _, pid := range pids {
		if err := v.Kill(pid); err != nil {
			log.Error("kill process failed", "pid", pid)
		}
	}
}

func (v *Vmm) IsExists(pid string) (ok bool) {
	v.vmsLockMu.RLock()
	defer v.vmsLockMu.RUnlock()

	_, ok = v.vms[pid]
	return
}

func (v *Vmm) GetVm(pid string) (vm schema.Vm, env *schema.Env, err error) {
	instance, err := v.getInstance(pid)
	if err != nil {
		return nil, nil, err
	}
	// Return an environment copy; operations on Vm must still use the task loop.
	err = v.call(instance, func(instance *schema.VmInstance) error {
		vm = instance.Vm
		copy := cloneEnv(*instance.Env)
		env = &copy
		return nil
	})
	return
}

func (v *Vmm) GetVmPids() (pids []string) {
	v.vmsLockMu.RLock()
	defer v.vmsLockMu.RUnlock()

	pids = make([]string, 0, len(v.vms))
	for pid := range v.vms {
		pids = append(pids, pid)
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

func (v *Vmm) GetVmCount() int64 {
	v.vmsLockMu.RLock()
	defer v.vmsLockMu.RUnlock()

	return int64(len(v.vms))
}

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

	locked, ok := v.vmsRecoveryLock[pid]
	if !ok {
		return false
	}
	return locked
}

func (v *Vmm) Checkpoint(pid string) (snap schema.Snapshot, err error) {
	instance, err := v.getInstance(pid)
	if err != nil {
		return
	}
	err = v.call(instance, func(instance *schema.VmInstance) error {
		snap.Data, snap.Err = instance.Vm.Checkpoint()
		snap.Env = cloneEnv(*instance.Env)
		return snap.Err
	})
	snap.Err = err
	return
}

func (v *Vmm) Restore(snap schema.Snapshot) error {
	snap.Env = cloneEnv(snap.Env)
	restore := func(instance *schema.VmInstance) error {
		if instance.Vm == nil {
			vm, err := v.spawn(cloneEnv(snap.Env))
			if err != nil {
				return err
			}
			instance.Vm = vm
		}
		if err := instance.Vm.Restore(snap.Data); err != nil {
			return err
		}
		instance.Env = &snap.Env
		return nil
	}
	instance, task, err := v.createInstance(snap.Env.Meta.Pid, restore)
	if err == schema.ErrProcessAlreadyExists {
		return v.call(instance, restore)
	}
	if err != nil {
		return err
	}
	<-task.Done
	if task.Err != nil {
		<-instance.Done
	}
	return task.Err
}

func (v *Vmm) addVm(vm schema.Vm, env *schema.Env) {
	_, task, err := v.createInstance(env.Meta.Pid, func(instance *schema.VmInstance) error {
		copy := cloneEnv(*env)
		instance.Vm = vm
		instance.Env = &copy
		return nil
	})
	if err == nil {
		<-task.Done
	}
}
