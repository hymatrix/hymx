package chainkit

import (
	"github.com/hymatrix/hymx/chainkit/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	hymxUtils "github.com/hymatrix/hymx/utils"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func (c *Chainkit) downloadByTxid(itemId string) (bundleItem *goarSchema.BundleItem, err error) {
	// Get from cache first
	bundleItem, err = c.db.GetCache(itemId)
	if err == nil {
		return bundleItem, nil
	}

	bundleItem, err = c.operator.Download(itemId)
	if err != nil {
		return nil, err
	}
	// Cache the downloaded bundle item
	err = c.db.Cache(itemId, *bundleItem)
	if err != nil {
		log.Error("cache bundle item failed", "itemId", itemId, "error", err)
	}
	return bundleItem, nil
}

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
		assignment, err := c.nodeDB.GetAssignByNonce(pid, nonce)
		if err != nil || assignment == nil {
			log.Debug("begin download", "assignId", assignId, "txId", txId)
			assignment, err = c.downloadByTxid(assignId)
			if err != nil {
				log.Error("download assignment failed", "assignId", assignId, "txId", txId, "error", err)
				continue
			}

			err = c.verifyMessage(assignment, hymxSchema.TypeAssignment)
			if err != nil {
				log.Error("verify assignment failed", "assignId", assignId, "txId", txId, "error", err)
				continue
			}
		}

		// check local db before download
		message, err := c.nodeDB.GetMessage(txId)
		if err != nil || message == nil {
			log.Debug("begin download message", "txId", txId)
			message, err = c.downloadByTxid(txId)
			if err != nil {
				log.Error("download message failed", "txId", txId, "error", err)
				continue
			}
		}

		if message == nil || assignment == nil {
			log.Error("downloadByNonce failed", "nonce", nonce, "assignId", assignId, "txId", txId)
			continue
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

func (c *Chainkit) verifyMessage(bundleItem *goarSchema.BundleItem, msgType string) error {
	// verify bundle item
	if err := goarUtils.VerifyBundleItem(*bundleItem); err != nil {
		return err
	}
	// get pid, signer, fromProcess, instance
	_, _, _, instance, err := hymxUtils.Decode(*bundleItem)
	if err != nil {
		return err
	}

	// verify Item type
	t := hymxUtils.GetType(*bundleItem)
	if t != msgType {
		return schema.ErrInvalidMessageType
	}
	switch msgType {
	case hymxSchema.TypeMessage:
		_, ok := instance.(hymxSchema.Message)
		if !ok {
			return schema.ErrInvalidMessageType
		}
	case hymxSchema.TypeProcess:
		_, ok := instance.(hymxSchema.Process)
		if !ok {
			return schema.ErrInvalidProcessType
		}
	case hymxSchema.TypeAssignment:
		_, ok := instance.(hymxSchema.Assignment)
		if !ok {
			return schema.ErrInvalidAssignmentType
		}
	default:
		return schema.ErrInvalidMessageType
	}

	// verify fromProcess
	// !!No strict verification of fromProcess is currently required

	// verify signer or scheduler
	// !!No strict verification of signer is currently required

	return nil
}
