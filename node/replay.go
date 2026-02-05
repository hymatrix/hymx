package node

import (
	"fmt"
	"sync"

	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func (n *Node) runReplay() error {
	allProcessId, err := n.sdk.Client.GetProcesses(n.info.Node.AccId)
	if err != nil {
		return err
	}

	pCount := len(allProcessId)
	var wg sync.WaitGroup
	wg.Add(pCount)

	// for each process id, create new vm and start it
	for i := 0; i < pCount; i++ {
		pid := allProcessId[i]
		maxNonce, err := n.getMaxNonce(pid)
		if err != nil {
			return err
		}

		err = n.recoveryTaskPool.Submit(func() {
			defer wg.Done()

			ckpId, err := n.db.GetCheckpointIndex(pid)
			if err != nil {
				log.Error("can not get checkpoint index", "pid", pid, "err", err)
			}

			if err := n.recoveryProcess(pid, maxNonce, ckpId, vmmSchema.ExecModeReplay); err != nil {
				log.Error("replay process error", "pid", pid, "maxNonce", maxNonce, "ckpId", ckpId, "err", err)
			}
		})
		if err != nil {
			log.Error("submit replay task error", "pid", pid, "maxNonce", maxNonce, "err", err)
		}
	}

	// wait all process recoverd
	wg.Wait()

	// finish replay
	log.Info("replay node successfully!")
	return nil
}

func (n *Node) getMaxNonce(pid string) (nonce int64, err error) {
	if n.chainkit == nil {
		return 0, fmt.Errorf("chainkit is nil")
	}
	return n.chainkit.GetMaxNonce(n.info.Node.AccId, pid)
}
