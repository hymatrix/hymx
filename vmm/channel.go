package vmm

import "github.com/hymatrix/hymx/vmm/schema"

func (v *Vmm) runInstance(pid string, instance *schema.VmInstance) {
	defer v.wg.Done()
	for {
		var task *schema.VmTask
		select {
		case task = <-instance.Tasks:
		case <-instance.Stopping:
			// A send racing with stop may still succeed. Finish all senders
			// before draining accepted tasks and executing the stop task.
			instance.Senders.Wait()
			select {
			case task = <-instance.Tasks:
			default:
				instance.Mu.Lock()
				task = instance.StopTask
				instance.Mu.Unlock()
			}
		}
		if task.Admitted != nil {
			<-task.Admitted
		}
		task.Err = task.Run(instance)
		// Failed creation must not leave an idle worker behind. In-flight
		// senders keep ownership of the instance until admission completes.
		if task.Err != nil && instance.Vm == nil {
			v.vmsLockMu.Lock()
			instance.Mu.Lock()
			if len(instance.Tasks) == 0 && instance.Sending == 0 && instance.StopTask == nil {
				instance.StopTask = task
				close(instance.Stopping)
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
