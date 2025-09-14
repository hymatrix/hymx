package chainkit

import (
	"strconv"
	"time"

	"github.com/hymatrix/hymx/chainkit/schema"
	goarSchema "github.com/permadao/goar/schema"
)

// uploadToChain implements Chainkit upload functionality
// Packages multiple BundleItems and uploads to blockchain network
// Returns parent transaction ID for subsequent status tracking
func (c *Chainkit) uploadToChain(txids []string) (parentTxID string, err error) {
	// Filter out already uploaded txids to avoid duplicates
	filteredTxids := c.filterUnuploadedTxids(txids)
	if len(filteredTxids) == 0 {
		log.Info("All txids already uploaded, skipping")
		return "", nil
	}

	items := c.getBundleItems(filteredTxids)
	if len(items) == 0 {
		return "", nil
	}

	parentTxID, err = c.operator.Upload(items)
	if err != nil {
		return "", err
	}

	// Add transactions to new parent
	if err = c.updateParentId(parentTxID, filteredTxids); err != nil {
		return "", err
	}

	return parentTxID, nil
}

// filterUnuploadedTxids filters out txids that have already been uploaded
func (c *Chainkit) filterUnuploadedTxids(txids []string) []string {
	var filteredTxids []string
	for _, txid := range txids {
		uploaded, err := c.isUploaded(txid)
		if err != nil {
			log.Error("Failed to check uploaded txid", "txid", txid, "err", err)
			continue
		}
		if !uploaded {
			filteredTxids = append(filteredTxids, txid)
		} else {
			log.Info("Skipping already uploaded txid", "txid", txid)
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
	// 1. Move transactions from upload set to pending set, set parentid to 0
	if err := c.moveToPending(); err != nil {
		log.Error("Failed to move uploads to pending", "err", err)
		return
	}

	// 2. Fetch transactions with parentid 0 and upload them, generate parentid and timestamp
	// 3. Check timed out parentids and re-upload them, generate new parentid and timestamp
	parentIDs, err := c.getPendingParentIDs()
	if err != nil {
		return
	}
	for _, parentID := range parentIDs {
		// Check if timed out (over 1 hour unconfirmed)
		// Or parentID is "0" which means never be uploaded
		if parentID == schema.ZeroParentID || c.checkTimeout(parentID) {
			txids, err := c.getPendingsByParentID(parentID)
			if err != nil {
				return
			}
			_, err = c.uploadToChain(txids)
			if err != nil {
				continue
			}
		}
		// 4. Remove confirmed transactions from pending set, clean up timestamp, record txid
		if parentID == schema.ZeroParentID {
			continue
		}
		ok, err := c.operator.CheckTransaction(parentID)
		if err == nil && ok {
			// Get all txids under this parent before removing

			txids, err := c.getPendingsByParentID(parentID)
			if err == nil {
				if err := c.addUploaded(txids); err != nil {
					log.Error("Failed to record uploaded txids", "txids", txids, "err", err)
				}
			}

			c.removePending(parentID)
		}
	}
}

// checkTimeout checks if parent transaction has timed out (over 1 hour unconfirmed)
func (c *Chainkit) checkTimeout(parentTxID string) bool {
	// Get parent transaction upload timestamp from Redis
	uploadTimeStr, err := c.redis.HGet(c.ctx, RedisKeyUploadTimestamp, parentTxID).Result()
	if err != nil {
		// No record or retrieval failed, possibly old transaction, do not process
		return false
	}

	uploadTime, err := strconv.ParseInt(uploadTimeStr, 10, 64)
	if err != nil {
		return false
	}

	// Check if over 1 hour has passed
	return time.Now().Unix()-uploadTime > 3600
}
