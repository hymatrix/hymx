package chainkit

import (
	"github.com/hymatrix/hymx/chainkit/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func (c *Chainkit) downloadByNonce(scheduler, pid string, beginNonce, endNonce int64) (results []*schema.DownloadResult, err error) {
	assignIds, txIds, err := c.queryByNonce(scheduler, pid, beginNonce, endNonce)
	if err != nil {
		return nil, err
	}

	log.Debug("downloadByNonce", "scheduler", scheduler, "pid", pid, "beginNonce", beginNonce, "endNonce", endNonce)
	log.Debug("downloadByNonce assignIds count", "count", len(assignIds))
	log.Debug("downloadByNonce txIds count", "count", len(txIds))

	for nonce := beginNonce; nonce <= endNonce; nonce++ {
		assignId, ok := assignIds[nonce]
		if !ok {
			//todo: if assignId exist? select which one ???
			continue
		}
		txId, ok := txIds[nonce]
		if !ok {
			continue
		}

		// check local db before download
		assignment, err := c.node.GetAssignByNonce(pid, nonce)
		if err != nil {
			log.Debug("begin download", "assignId", assignId, "txId", txId)
			assignment, err = c.DownloadByTxid(assignId)
			if err != nil {
				return nil, err
			}

			err = c.verifyAssignment(assignment)
			if err != nil {
				log.Debug("verify assignment failed", "error", err)
				continue
			}
		}

		// check local db before download
		message, err := c.node.GetMessage(txId)
		if err != nil {
			log.Debug("begin download message", "txId", txId)
			message, err = c.DownloadByTxid(txId)
			if err != nil {
				return nil, err
			}

			// todo: commit to local cache
		}

		results = append(results, &schema.DownloadResult{
			Nonce:      nonce,
			Assignment: assignment,
			Message:    message,
		})
	}
	return results, nil
}

func (c *Chainkit) downloads(itemsIds []string) (items []*goarSchema.BundleItem, err error) {
	items, err = c.operator.Downloads(itemsIds)
	return items, err
}

func (c *Chainkit) verifySpawnMsg(spawnMsg *goarSchema.BundleItem) error {
	// verify bundle item
	if err := goarUtils.VerifyBundleItem(*spawnMsg); err != nil {
		return err
	}
	pid, _, _, instance, err := utils.Decode(*spawnMsg)
	if err != nil {
		return err
	}
	// verify fromProcess
	// if err = c.node.VerifyFromProcess(*spawnMsg, pid, signer, fromProcess); err != nil {
	// 	return err
	// }
	// verify Scheduler is valid
	proc, ok := instance.(hymxSchema.Process)
	if !ok {
		return schema.ErrInvalidProcessType
	}
	// verify Scheduler
	if err = c.node.AuthNode(proc.Scheduler, pid); err != nil {
		return err
	}
	return nil
}

func (c *Chainkit) verifyAssignment(assignmentItem *goarSchema.BundleItem) error {
	// verify bundle item
	if err := goarUtils.VerifyBundleItem(*assignmentItem); err != nil {
		return err
	}
	// get pid, signer, fromProcess, instance
	_, _, _, instance, err := utils.Decode(*assignmentItem)
	if err != nil {
		return err
	}
	// verify fromProcess
	// if err = c.node.VerifyFromProcess(*assignmentItem, pid, signer, fromProcess); err != nil {
	// 	return err
	// }
	// verify Item type
	_, ok := instance.(hymxSchema.Assignment)
	if !ok {
		return schema.ErrInvalidAssignmentType
	}
	// verify signer
	// if err = c.node.AuthNode(signer, pid); err != nil {
	// 	return err
	// }

	return nil
}
