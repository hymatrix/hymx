package chainkit

import (
	"fmt"
	"strconv"
	"time"

	goarSchema "github.com/permadao/goar/schema"
)

// uploadToChain implements Chainkit upload functionality
// Packages multiple BundleItems and uploads to blockchain network
// Returns parent transaction ID for subsequent status tracking
func (c *Chainkit) uploadToChain(items []goarSchema.BundleItem) (parentTxID string, err error) {
	log.Info("uploading bundle items", "count", len(items))
	return c.operator.Upload(items)
}

// Implements Chainkit transaction aggregation functionality
// Gets all pending upload txids from uploadSet
// Gets all transactions from idb
// Uses goar
// Uses operator.Upload for uploading
// If successful, removes from uploadSet and adds to uploadingSet (records parent transaction ID)
// If failed, waits for next aggregation
func (c *Chainkit) aggregate() (string, error) {
	// Collect all currently pending sub transactions
	c.mu.Lock()
	txids, err := c.getUploads()
	if err != nil {
		c.mu.Unlock()
		return "", err
	}
	if len(txids) == 0 {
		c.mu.Unlock()
		return "", nil
	}
	c.mu.Unlock()

	items := c.getBundleItems(txids)
	if len(items) == 0 {
		return "", nil
	}

	// Call operator.Upload for aggregated upload (operator implements specific packaging logic)
	parentTxID, err := c.uploadToChain(items)
	if err != nil {
		return "", err
	}

	// Success: remove from pending upload, add to uploading set
	uploaded := make([]string, len(items))
	for _, item := range items {
		uploaded = append(uploaded, item.Id)
	}
	if err = c.moveToPending(parentTxID, uploaded); err != nil {
		return "", err
	}

	return parentTxID, nil
}

// Execute aggregation tasks in a goroutine
// Aggregate according to aggregationPolicy (time condition only)
func (c *Chainkit) tryByTime() {
	interval := c.aggregationPolicy.MaxDelay // second
	ticker := time.NewTicker(interval * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			go c.aggregate()
		}
	}
}

func (c *Chainkit) tryByCount() {
	n, err := c.getUploadsCount()
	if err != nil {
		return
	}
	if n >= c.aggregationPolicy.MaxItems {
		go c.aggregate()
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

// reupload re-uploads timed out parent transactions
func (c *Chainkit) reupload(parentTxID string) (string, error) {
	log.Debug("reupload parent transaction", "txid", parentTxID)

	// Get all sub transaction IDs under this parent transaction
	subTxIDs, err := c.getPendingSub(parentTxID)
	if err != nil {
		return "", fmt.Errorf("failed to get pending sub transactions: %w", err)
	}

	// Re-aggregate these sub transactions
	items := c.getBundleItems(subTxIDs)
	if len(items) == 0 {
		return "", fmt.Errorf("no valid items to reupload")
	}

	// Re-upload
	newParentTxID, err := c.uploadToChain(items)
	if err != nil {
		log.Error("Failed to reupload parent transaction", "txid", parentTxID, "err", err)
		return "", fmt.Errorf("failed to upload to chain: %w", err)
	}

	// Update Redis records: remove old parent transaction record, add new one
	// Remove old record
	if err := c.removePending(parentTxID); err != nil {
		return "", fmt.Errorf("failed to remove pending record: %w", err)
	}
	// Add new record
	uploaded := make([]string, len(items))
	for _, item := range items {
		uploaded = append(uploaded, item.Id)
	}
	if err := c.moveToPending(newParentTxID, uploaded); err != nil {
		return "", fmt.Errorf("failed to move to pending: %w", err)
	}

	log.Debug("reuploaded with new parent txid", "txid", newParentTxID)
	return newParentTxID, nil
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

// Check transaction status every 5 minutes (check if parent transaction is confirmed)
func (c *Chainkit) check() {
	interval := 5 * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// Get all pending parent transaction IDs
			parentIDs, err := c.getPendings()
			if err != nil {
				continue
			}
			for _, txid := range parentIDs {
				// Check if timed out (over 1 hour unconfirmed)
				if c.checkTimeout(txid) {
					if _, err := c.reupload(txid); err == nil {
						continue // Already re-uploaded, skip current check
					}
				}

				ok, err := c.operator.CheckTransaction(txid)
				if err == nil && ok {
					c.removePending(txid)
				}
			}
		}
	}
}
