package node

import (
	"errors"
	"sync"

	"github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/utils"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
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
	log.Debug("recovery process", "pid", pid, "maxNonce", maxNonce, "ckpId", ckpId)
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
			msg, assignItem, err := n.getMessageAndAssignByNonce(pid, nonce)
			if err != nil {
				return err
			}

			assign, err := utils.TagsToAssignment(assignItem.Tags)
			if err != nil {
				return err
			}

			if err = n.HandleMode(*msg, assign, vmmSchema.ExecModeDryRun, maxNonce); err != nil {
				log.Error("handle message failed in recover", "nonce", nonce, "err", err)
				continue
			}
		}

	}
	return nil
}

// getMessageAndAssignByNonce retrieves both message and assignment by process ID and nonce
func (n *Node) getMessageAndAssignByNonce(pid string, nonce int64) (msgItem, assignItem *goarSchema.BundleItem, err error) {
	// First try to get from local database
	msgItem, err1 := n.db.GetMessageByNonce(pid, nonce)
	assignItem, err2 := n.db.GetAssignByNonce(pid, nonce)

	// If both are available locally, return them
	if err1 == nil || err2 == nil || msgItem != nil || assignItem != nil {
		return msgItem, assignItem, nil
	}

	// If either is missing, try to download once
	result, err := n.chainkit.DownloadByPid(pid, nonce, nonce)
	if err != nil {
		return nil, nil, err
	}
	msgItem = result[0].Message
	assignItem = result[0].Assignment

	return msgItem, assignItem, nil
}
