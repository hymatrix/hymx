package node

import (
	"errors"
	"sync"

	"github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/utils"
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

// getMessageAndAssignByNonce retrieves both message and assignment by process ID and nonce
func (n *Node) getMessageAndAssignByNonce(pid string, nonce int64) (msgItem, assignItem *goarSchema.BundleItem, err error) {
	// Retry logic - attempt up to 3 times
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		msgItem, err1 := n.db.GetMessageByNonce(pid, nonce)
		assignItem, err2 := n.db.GetAssignByNonce(pid, nonce)

		if err1 != nil || err2 != nil {
			// If either fetch fails, try to download and retry
			_, err = n.chainkit.DownloadByPid(pid, nonce, nonce)
			if err != nil {
				// If download also fails, continue to next retry or return error
				if attempt == maxRetries-1 {
					// Last attempt failed, return the error
					return nil, nil, err
				}
				// Not the last attempt, continue to retry
				continue
			}
			// Download succeeded, continue to next iteration to retry fetching
			continue
		}

		// Both fetches succeeded, return the results
		return msgItem, assignItem, nil
	}

	// All retries exhausted, return error
	return nil, nil, errors.New("failed to get message and assignment after 3 retries")
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

	_, err := n.chainkit.DownloadByPid(pid, beginNonce, maxNonce)
	if err != nil {
		return err
	}

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

			if err = n.HandleDryRun(*msg, assign, maxNonce); err != nil {
				log.Error("handle message failed in recover", "nonce", nonce, "err", err)
				continue
			}
		}

	}
	return nil
}
