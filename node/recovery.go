package node

import (
	"errors"
	"sync"

	"github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/utils"
)

func (n *Node) runRecovery() error {
	// get all process id and nonce from db
	allProcessId, allNonce, err := n.db.GetAllProcess()
	if err != nil {
		return err
	}

	pCount := len(allProcessId)
	var wg sync.WaitGroup
	wg.Add(pCount)

	// for each process id, create new vm and start it
	for i := 0; i < pCount; i++ {
		pid := allProcessId[i]
		maxNonce := allNonce[i]

		err := n.recoveryTaskPool.Submit(func() {
			defer wg.Done()

			ckpId, err := n.db.GetCheckpointIndex(pid)
			if err != nil {
				log.Error("can not get checkpoint index", "pid", pid, "err", err)
			}

			if err := n.recoveryProcess(pid, maxNonce, ckpId); err != nil {
				log.Error("recovery process error", "pid", pid, "maxNonce", maxNonce, "ckpId", ckpId, "err", err)
			}
		})
		if err != nil {
			log.Error("submit recovery task error", "pid", pid, "maxNonce", maxNonce, "err", err)
		}
	}

	// wait all process recoverd
	wg.Wait()

	// finish recovery
	log.Info("recovery node successfully!")
	return nil
}

func (n *Node) recoveryProcess(pid string, maxNonce int64, ckpId string) error {
	if n.vmm.IsRecovering(pid) {
		return schema.ErrProcessIsRecovering
	}
	// lock process
	n.vmm.RecoveryLock(pid)

	// init nonce to 0
	beginNonce := int64(0)

	// load checkpoint if exist
	if nonce, err := n.Restore(ckpId); err == nil {
		if nonce == maxNonce {
			n.vmm.RecoveryUnlock(pid)
			return nil
		}

		beginNonce = nonce + 1
	}

	n.wg.Add(1)
	defer n.wg.Done()
	// recovering all message of the process
	for nonce := beginNonce; nonce <= maxNonce; nonce++ {
		select {
		case <-n.ctx.Done():
			return errors.New("node closed")
		default:
			msg, err := n.db.GetMessageByNonce(pid, nonce)
			if err != nil {
				return err
			}

			assignItem, err := n.db.GetAssignByNonce(pid, nonce)
			if err != nil {
				return err
			}

			assign, err := utils.TagsToAssignment(assignItem.Tags)
			if err != nil {
				return err
			}

			if err = n.HandleDryRun(*msg, assign, maxNonce); err != nil {
				log.Error("handle message failed in recover", "nonce", nonce, "err", err)
				continue
			}
		}

	}
	return nil
}
