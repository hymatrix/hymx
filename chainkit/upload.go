package chainkit

import (
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func (c *Chainkit) uploadToChain(txids []string) (bundledInId string, uploaded []string, err error) {
	log.Debug("chainkit enter uploadToChain", "count", len(txids), "txids", txids)

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

	// gen binary
	binaryItems := []goarSchema.BundleItem{}
	uploadTxids := []string{}
	for _, item := range items {
		item.Binary, err = goarUtils.GenerateItemBinary(item)
		if err != nil {
			log.Error("Failed to generate item binary", "item", item.Id, "err", err)
			continue
		}
		binaryItems = append(binaryItems, item)
		uploadTxids = append(uploadTxids, item.Id)
	}

	bundledInId, err = c.operator.Upload(binaryItems)
	if err != nil {
		log.Error("Failed to upload txids", "err", err)
		return "", []string{}, err
	}

	return bundledInId, uploadTxids, nil
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
	// items := make([]goarSchema.BundleItem, 0, len(txids)*2)
	items := []goarSchema.BundleItem{}
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
	// Lock to prevent concurrent execution
	c.mu.Lock()
	defer c.mu.Unlock()

	log.Debug("chainkit enter check")

	curBundledIn, err := c.getBundledIn()
	if err != nil {
		log.Error("Failed to get current bundledIn", "err", err)
		return
	}

	if curBundledIn != "" {
		ok, err := c.operator.CheckTransaction(curBundledIn)
		if err != nil {
			log.Error("Failed to check transaction", "bundledInID", curBundledIn, "err", err)
			return
		}
		if !ok {
			log.Debug("Transaction not confirmed, skip", "bundledInID", curBundledIn)
			return
		}
		log.Debug("Transaction confirmed, move to uploaded", "bundledInID", curBundledIn)

		// transaction confirmed move to uploaded
		err = c.endUpload()
		if err != nil {
			log.Error("Failed to end upload", "bundledInID", curBundledIn, "err", err)
			return
		}
	}

	c.tryUpload()
}

func (c *Chainkit) tryUpload() error {
	log.Debug("chainkit tryUpload")

	curBundledIn, err := c.getBundledIn()
	if err != nil {
		log.Error("Failed to get current bundledIn", "err", err)
		return err
	}

	if curBundledIn != "" {
		return nil
	}

	// move to uploading
	c.moveToUploading()

	uploadingTxids, err := c.getUploading()
	if err != nil {
		log.Error("Failed to get uploading txids", "err", err)
		return err
	}

	if len(uploadingTxids) == 0 {
		log.Debug("No txids to upload", "count", len(uploadingTxids))
		return nil
	}

	// upload
	bundledInId, uploaded, err := c.uploadToChain(uploadingTxids)
	if err != nil {
		log.Error("Failed to upload txids", "err", err)
		return err
	}

	if len(uploaded) == 0 {
		log.Debug("No txids uploaded", "count", len(uploaded))
		return nil
	}

	// save bundledIn
	err = c.setBundledIn(bundledInId)
	if err != nil {
		log.Error("Failed to set bundledIn", "bundledInID", bundledInId, "err", err)
		return err
	}

	return nil
}
