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
	vm, _, err := v.GetVm(pid)
	if err != nil {
		return
	}

	v.vmsLockMu.Lock()
	defer v.vmsLockMu.Unlock()
	vm.Close()
	delete(v.vms, pid)
	delete(v.vmsEnv, pid)

	return
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
	v.vmsLockMu.RLock()
	defer v.vmsLockMu.RUnlock()

	ok := false
	if vm, ok = v.vms[pid]; !ok {
		err = schema.ErrProcessNotFound
		return
	}
	if env, ok = v.vmsEnv[pid]; !ok {
		err = schema.ErrProcessEnvNotFound
	}
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
	if !v.IsExists(pid) {
		err = schema.ErrProcessNotFound
		return
	}

	res := make(chan schema.Snapshot)
	defer close(res)

	v.ckpChan <- schema.Checkpoint{
		Pid: pid,
		Res: res,
	}

	snap = <-res
	if snap.Err != nil {
		err = snap.Err
	}
	return
}

func (v *Vmm) Restore(snap schema.Snapshot) error {
	vm, _, err := v.GetVm(snap.Env.Id)
	if err != nil {
		if vm, err = v.spawn(snap.Env); err != nil {
			return err
		}
	}

	if err = vm.Restore(snap.Data); err != nil {
		return err
	}
	v.addVm(vm, &snap.Env)
	return nil
}

func (v *Vmm) checkpoint(pid string, res chan<- schema.Snapshot) {
	vm, env, err := v.GetVm(pid)
	if err != nil {
		res <- schema.Snapshot{
			Err: err,
		}
		return
	}

	data, err := vm.Checkpoint()
	if err != nil {
		res <- schema.Snapshot{
			Err: err,
		}
		return
	}

	res <- schema.Snapshot{
		Env:  *env,
		Data: data,
	}
}

func (v *Vmm) addVm(vm schema.Vm, env *schema.Env) {
	v.vmsLockMu.Lock()
	defer v.vmsLockMu.Unlock()

	v.vms[env.Id] = vm
	v.vmsEnv[env.Id] = env
}
