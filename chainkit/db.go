package chainkit

import "time"

const (
	RedisUrl                = "redis://@localhost:6379/0"
	RedisKeyUploads         = "chainkit:uploads"
	RedisKeyPendingPrefix   = "chainkit:pending:"          // Hash key prefix: chainkit:pending:{parentTxID}
	RedisKeyUploadTimestamp = "chainkit:upload_timestamps" // Hash key for storing upload timestamps

	// todo: record uploaded txid
)

// Redis operation wrapper functions

// getUploads gets all members from the upload set
func (c *Chainkit) getUploads() ([]string, error) {
	return c.redis.SMembers(c.ctx, RedisKeyUploads).Result()
}

// getUploadsCount gets the count of members in the upload set
func (c *Chainkit) getUploadsCount() (int64, error) {
	return c.redis.SCard(c.ctx, RedisKeyUploads).Result()
}

// addToUploads adds transaction ID to the upload set
func (c *Chainkit) addToUploads(txid string) error {
	return c.redis.SAdd(c.ctx, RedisKeyUploads, txid).Err()
}

// moveToPending removes transaction ID from upload set and adds to pending hash set
func (c *Chainkit) moveToPending(parentTxID string, txids []string) error {
	pipe := c.redis.TxPipeline()
	for _, txid := range txids {
		pipe.SRem(c.ctx, RedisKeyUploads, txid)
		// Add sub txid to hash with parentTxID as key
		pipe.HSet(c.ctx, RedisKeyPendingPrefix+parentTxID, txid, "1")
	}
	// Record upload timestamp of parent transaction
	pipe.HSet(c.ctx, RedisKeyUploadTimestamp, parentTxID, time.Now().Unix())
	_, err := pipe.Exec(c.ctx)
	return err
}

// getPendings gets all pending parent transaction IDs
func (c *Chainkit) getPendings() ([]string, error) {
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

// getPendingSub gets all sub transaction IDs by parent transaction ID
func (c *Chainkit) getPendingSub(parentTxID string) ([]string, error) {
	return c.redis.HKeys(c.ctx, RedisKeyPendingPrefix+parentTxID).Result()
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
