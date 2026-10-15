package node

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func (n *Node) recoverProcesses(pids []string, nonces []int64, mode vmmSchema.ExecMode) error {
	plan, planErr := n.recoveryOrder(pids)
	var wg sync.WaitGroup
	var failureMu sync.Mutex
	var registryFailed atomic.Bool
	failures := []error{planErr}
	recordFailure := func(index int, err error) {
		pid := pids[index]
		if plan.registryTasks[index] {
			registryFailed.Store(true)
		}
		log.Error("skipping recovery process", "pid", pid, "mode", mode, "err", err)
		failureMu.Lock()
		failures = append(failures, fmt.Errorf("recover process %s: %w", pid, err))
		failureMu.Unlock()
	}
	registryChecked, registryReady := false, false
	for position, index := range plan.order {
		pid := pids[index]
		if position >= plan.coreCount {
			if !registryChecked {
				// Even with multiple workers, no ordinary VM can occupy a worker
				// before all scheduled core tasks have finished.
				wg.Wait()
				registryReady = !registryFailed.Load() && n.recoveryRegistryReady()
				registryChecked = true
			}
			if !registryReady {
				recordFailure(index, vmmSchema.ErrRegistryNotFound)
				continue
			}
		}
		var maxNonce int64
		if nonces == nil {
			var err error
			maxNonce, err = n.getMaxNonce(pid)
			if err != nil {
				recordFailure(index, err)
				continue
			}
		} else if index >= len(nonces) {
			recordFailure(index, fmt.Errorf("stored nonce unavailable"))
			continue
		} else {
			maxNonce = nonces[index]
		}
		wg.Add(1)
		err := n.recoveryTaskPool.Submit(func() {
			defer wg.Done()
			checkpointID, err := n.db.GetCheckpointIndex(pid)
			if err != nil {
				log.Warn("checkpoint index unavailable, trying transaction recovery", "pid", pid, "err", err)
			}
			if err := n.recoveryProcess(pid, maxNonce, checkpointID, mode); err != nil {
				recordFailure(index, err)
			}
		})
		if err != nil {
			wg.Done()
			recordFailure(index, err)
		}
	}
	wg.Wait()
	log.Info("node recovery pass finished", "mode", mode, "total", len(pids), "eligible", len(plan.order))
	return errors.Join(failures...)
}

func (n *Node) recoveryRegistryReady() bool {
	select {
	case <-n.registrySpawned:
		return true
	default:
	}
	// A joined node may be asynchronously forking a remote registry. Allow a
	// bounded wait outside the worker pool; a failed registry cannot hang it.
	if n.info == nil || n.info.Registry == "" || n.registrySpawned == nil {
		return false
	}
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-n.registrySpawned:
		return true
	case <-n.ctx.Done():
		return false
	case <-timer.C:
		return false
	}
}
