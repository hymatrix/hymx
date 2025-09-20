package chainkit

import (
	"github.com/hymatrix/hymx/chainkit/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func (c *Chainkit) downloads(itemsIds []string) (items []*goarSchema.BundleItem, err error) {
	items, err = c.operator.Downloads(itemsIds)
	return items, err
}

func (c *Chainkit) verifySpawnMsg(spawnMsg *goarSchema.BundleItem) error {
	// verify bundle item
	if err := goarUtils.VerifyBundleItem(*spawnMsg); err != nil {
		return err
	}
	pid, signer, fromProcess, instance, err := utils.Decode(*spawnMsg)
	if err != nil {
		return err
	}
	// verify fromProcess
	if err = c.node.VerifyFromProcess(*spawnMsg, pid, signer, fromProcess); err != nil {
		return err
	}
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
	pid, signer, fromProcess, instance, err := utils.Decode(*assignmentItem)
	if err != nil {
		return err
	}
	// verify fromProcess
	if err = c.node.VerifyFromProcess(*assignmentItem, pid, signer, fromProcess); err != nil {
		return err
	}
	// verify Item type
	_, ok := instance.(hymxSchema.Assignment)
	if !ok {
		return schema.ErrInvalidAssignmentType
	}
	// verify signer
	if err = c.node.AuthNode(signer, pid); err != nil {
		return err
	}

	return nil
}

func (c *Chainkit) downloadByNonce(scheduler, pid string, beginNonce, endNonce int64) (results []*schema.DownloadResult, err error) {
	assignIds, txIds, err := c.queryByNonce(scheduler, pid, beginNonce, endNonce)
	if err != nil {
		return nil, err
	}

	for i := beginNonce; i <= endNonce; i++ {
		assignId, ok := assignIds[i]
		if !ok {
			continue
		}
		txId, ok := txIds[i]
		if !ok {
			continue
		}

		assignment, err := c.DownloadByTxid(assignId)
		if err != nil {
			return nil, err
		}

		err = c.verifyAssignment(assignment)
		if err != nil {
			continue
		}

		message, err := c.DownloadByTxid(txId)
		if err != nil {
			return nil, err
		}

		results = append(results, &schema.DownloadResult{
			Nonce:      i,
			Assignment: assignment,
			Message:    message,
		})
	}
	return results, nil
}
