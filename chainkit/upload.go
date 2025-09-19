package chainkit

import (
	"time"

	goarSchema "github.com/permadao/goar/schema"
)

func (c *Chainkit) uploadToChain(txids []string) (bundledInId string, uploaded []string, err error) {
	// Filter out already uploaded txids to avoid duplicates
	filteredTxids := c.filterUnuploadedTxids(txids)
	if len(filteredTxids) == 0 {
		log.Info("All txids already uploaded, skipping")
		return "", []string{}, nil
	}

	items := c.getBundleItems(filteredTxids)
	if len(items) == 0 {
		return "", filteredTxids, nil
	}

	bundledInId, err = c.operator.Upload(items)
	if err != nil {
		return "", []string{}, err
	}

	return
}

// filterUnuploadedTxids filters out txids that have already been uploaded
func (c *Chainkit) filterUnuploadedTxids(txids []string) []string {
	uploadedMap, err := c.isUploadedBatch(txids)
	if err != nil {
		log.Error("Failed to check uploaded txids in batch", "err", err)
		return txids
	}

	var filteredTxids []string
	for _, txid := range txids {
		if !uploadedMap[txid] {
			filteredTxids = append(filteredTxids, txid)
		}
	}
	return filteredTxids
}

// getBundleItems collects BundleItem data from given transaction ID list
func (c *Chainkit) getBundleItems(txids []string) []goarSchema.BundleItem {
	items := make([]goarSchema.BundleItem, 0, len(txids)*2)
	for _, txid := range txids {
		if msg, err := c.node.GetMessage(txid); err == nil && msg != nil {
			items = append(items, *msg)
		}
		if assign, err := c.node.GetAssignByMessage(txid); err == nil && assign != nil {
			items = append(items, *assign)
		}
	}
	return items
}

func (c *Chainkit) check() {
	// 1. Check BundledInStatus
	c.checkBundledInStatus()

	// 2. Check if transaction is confirmed
	c.checkUploadStatus()
}

func (c *Chainkit) checkBundledInStatus() {
	// If pending status, skip
	// If timeout status, retransmit
	// If empty status, create new bundledIn
	status, statusStruct, err := c.getBundledInStatus()
	if err != nil {
		log.Error("Failed to get BundledInStatus", "err", err)
		return
	}
	switch status {
	case BundledInStatusEmpty:
		// Create new bundledIn
		pending, err := c.getPendingTxs()
		if err != nil {
			log.Error("Failed to get pending txs", "err", err)
			return
		}
		if len(pending) > 0 {
			bundledIn, uploaded, err := c.uploadToChain(pending)
			if err != nil {
				log.Error("Failed to upload pending txs", "err", err)
				return
			}
			err = c.createBundledIn(bundledIn, uploaded, time.Now().Unix())
			if err != nil {
				log.Error("Failed to create bundledIn", "err", err)
				return
			}
		}
	case BundledInStatusPending:
		// Skip
		log.Debug("BundledInStatusPending, skip", "bundledInID", statusStruct.BundledInID)
	case BundledInStatusTimeout:
		// Retransmit
		toReUpdates := statusStruct.TxIds
		// First add pending pool transactions to upload list
		pending, err := c.getPendingTxs()
		if err != nil {
			log.Error("Failed to get pending txs", "err", err)
		}
		if len(pending) > 0 {
			toReUpdates = append(toReUpdates, pending...)
		}
		if len(toReUpdates) > 0 {
			bundledIn, uploaded, err := c.uploadToChain(toReUpdates)
			if err != nil {
				log.Error("Failed to upload txs", "err", err)
				return
			}
			err = c.updateBundledIn(bundledIn, uploaded, time.Now().Unix())
			if err != nil {
				log.Error("Failed to update bundledIn", "err", err)
				return
			}
		}
	}
}

func (c *Chainkit) checkUploadStatus() {
	// If confirmed, delete BundledIn and upload pending transactions
	// If not confirmed, return
	_, statusStruct, err := c.getBundledInStatus()
	if err != nil {
		log.Error("Failed to get BundledInStatus", "err", err)
		return
	}
	ok, err := c.operator.CheckTransaction(statusStruct.BundledInID)
	if err != nil {
		log.Error("Failed to check transaction", "err", err)
		return
	}
	if ok {
		// Delete BundledIn
		err = c.deleteBundledIn(statusStruct.BundledInID)
		if err != nil {
			log.Error("Failed to delete bundledIn", "err", err)
			return
		}
		// Mark these txids as successfully uploaded
		err = c.addUploaded(statusStruct.TxIds)
		if err != nil {
			log.Error("Failed to add uploaded txids", "err", err)
			return
		}
	}
}
