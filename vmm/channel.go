package vmm

import "github.com/hymatrix/hymx/vmm/schema"

func (v *Vmm) runInstance(pid string, instance *schema.VmInstance) {
	defer v.wg.Done()
	for range instance.Wake {
		for {
			instance.Mu.Lock()
			if len(instance.Tasks) == 0 {
				instance.Mu.Unlock()
				break
			}
			task := instance.Tasks[0]
			instance.Tasks[0] = nil
			instance.Tasks = instance.Tasks[1:]
			if len(instance.Tasks) == 0 {
				instance.Tasks = nil
			}
			instance.Mu.Unlock()
			task.Err = task.Run(instance)
			// Failed creation must not leave an idle worker behind. Mark the
			// detached instance stopped so concurrent submitters cannot orphan work.
			if task.Err != nil && instance.Vm == nil {
				v.vmsLockMu.Lock()
				instance.Mu.Lock()
				if len(instance.Tasks) == 0 && instance.StopTask == nil {
					instance.StopTask = task
					task.Stop = true
					delete(v.vms, pid)
				}
				instance.Mu.Unlock()
				v.vmsLockMu.Unlock()
			}
			if task.Done != nil {
				close(task.Done)
			} else if task.Err != nil {
				log.Error("apply failed", "err", task.Err)
			}
			if task.Stop {
				return
			}
		}
	}
}
