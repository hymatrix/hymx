package chainkit

import (
	"time"

	"github.com/hymatrix/hymx/chainkit/schema"
)

const (
	RedisUrl                = "redis://@localhost:6379/0"
	RedisKeyUploads         = "chainkit:uploads"
	RedisKeyPendingPrefix   = "chainkit:pending:"          // Hash key prefix: chainkit:pending:{parentTxID}
	RedisKeyUploadTimestamp = "chainkit:upload_timestamps" // Hash key for storing upload timestamps
	RedisKeyUploadedTxids   = "chainkit:uploaded_txids"    // Set key for storing uploaded transaction IDs
)

// Redis operation wrapper functions

// getUploadsCount gets the count of members in the upload set
func (c *Chainkit) getUploadsCount() (int64, error) {
	return c.redis.SCard(c.ctx, RedisKeyUploads).Result()
}

// getUploads gets all members from the upload set
func (c *Chainkit) getUploads() ([]string, error) {
	return c.redis.SMembers(c.ctx, RedisKeyUploads).Result()
}

// addToUploads adds transaction ID to the upload set
func (c *Chainkit) addToUploads(txid string) error {
	return c.redis.SAdd(c.ctx, RedisKeyUploads, txid).Err()
}

// moveToPending removes transaction ID from upload set and adds to pending hash set
func (c *Chainkit) moveToPending() error {
	// 0 means never be uploaded
	parentTxID := schema.ZeroParentID
	txids, err := c.redis.SMembers(c.ctx, RedisKeyUploads).Result()
	if err != nil {
		return err
	}
	pipe := c.redis.TxPipeline()
	for _, txid := range txids {
		pipe.SRem(c.ctx, RedisKeyUploads, txid)
		// Add sub txid to hash with parentTxID as key
		pipe.HSet(c.ctx, RedisKeyPendingPrefix+parentTxID, txid, "1")
	}
	// Record upload timestamp of parent transaction
	//pipe.HSet(c.ctx, RedisKeyUploadTimestamp, parentTxID, time.Now().Unix())
	_, err = pipe.Exec(c.ctx)
	return err
}

func (c *Chainkit) updateParentId(newParentTxID string, txids []string) error {
	pipe := c.redis.TxPipeline()

	// Find and delete old parent transaction data for each txid
	oldParentIDs := make(map[string]bool) // Use map to avoid duplicate deletions
	for _, txid := range txids {
		oldParentTxID, err := c.findParentByTxid(txid)
		if err != nil {
			continue // Skip if error finding parent
		}
		if oldParentTxID != "" && oldParentTxID != newParentTxID {
			oldParentIDs[oldParentTxID] = true
		}
	}

	// Delete all old parent transaction data
	for oldParentTxID := range oldParentIDs {
		pipe.Del(c.ctx, RedisKeyPendingPrefix+oldParentTxID)
		pipe.HDel(c.ctx, RedisKeyUploadTimestamp, oldParentTxID)
	}

	// Add transactions to new parent
	for _, txid := range txids {
		pipe.HSet(c.ctx, RedisKeyPendingPrefix+newParentTxID, txid, "1")
	}
	// Record upload timestamp of parent transaction
	pipe.HSet(c.ctx, RedisKeyUploadTimestamp, newParentTxID, time.Now().Unix())
	_, err := pipe.Exec(c.ctx)
	return err
}

// getPendings gets all pending parent transaction IDs
func (c *Chainkit) getPendingParentIDs() ([]string, error) {
	keys, err := c.redis.Keys(c.ctx, RedisKeyPendingPrefix+"*").Result()
	if err != nil {
		return nil, err
	}
	// Remove prefix, only return parentTxID
	parentTxIDs := make([]string, len(keys))
	for i, key := range keys {
		parentTxIDs[i] = key[len(RedisKeyPendingPrefix):]
	}
	return parentTxIDs, nil
}

func (c *Chainkit) getPendingsByParentID(parentTxID string) ([]string, error) {
	return c.redis.HKeys(c.ctx, RedisKeyPendingPrefix+parentTxID).Result()
}

// findParentByTxid finds the parent transaction ID for a given sub transaction ID
func (c *Chainkit) findParentByTxid(txid string) (string, error) {
	// Get all pending parent IDs
	parentIDs, err := c.getPendingParentIDs()
	if err != nil {
		return "", err
	}

	// Search through each parent to find which one contains this txid
	for _, parentID := range parentIDs {
		exists, err := c.redis.HExists(c.ctx, RedisKeyPendingPrefix+parentID, txid).Result()
		if err != nil {
			continue
		}
		if exists {
			return parentID, nil
		}
	}

	return "", nil // Not found
}

// removePending removes entire parent transaction and all its sub transactions
func (c *Chainkit) removePending(parentTxID string) error {
	pipe := c.redis.TxPipeline()
	// Delete pending record
	pipe.Del(c.ctx, RedisKeyPendingPrefix+parentTxID)
	// Clean up upload time record
	pipe.HDel(c.ctx, RedisKeyUploadTimestamp, parentTxID)
	_, err := pipe.Exec(c.ctx)
	return err
}

// isUploadedTxid checks if a transaction ID has already been uploaded
func (c *Chainkit) isUploaded(txid string) (bool, error) {
	return c.redis.SIsMember(c.ctx, RedisKeyUploadedTxids, txid).Result()
}

func (c *Chainkit) addUploaded(txids []string) error {
	return c.redis.SAdd(c.ctx, RedisKeyUploadedTxids, txids).Err()
}
