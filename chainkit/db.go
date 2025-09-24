package chainkit

import (
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	RedisUrl = "redis://@localhost:6379/0"

	RdbPendingTxIds     = "chainkit:pending"              // List: Pending TxId FIFO queue
	RdbUploadingTxIds   = "chainkit:uploading"            // Set: Uploading TxId pool
	RdbCurrentBundledIn = "chainkit:current_bundledin_id" // String: current bundledIn id with 1 hour expiration
	RdbUploadedTxIds    = "chainkit:uploaded_txids"       // Set: Uploaded TxId pool
)

const (
	// MaxUploadingCount is the maximum number of transactions allowed in uploading state
	MaxUploadingCount = 100
)

var (
	ErrBundledInAlreadyExists = errors.New("bundledIn_already_exists")
	ErrTxIdsEmpty             = errors.New("txids_is_empty")
)

// BundledInStatusEnum represents the current bundledIn status
type BundledInStatusEnum string

const (
	BundledInStatusEmpty   BundledInStatusEnum = "empty"   // No RdbCurrentBundledIn
	BundledInStatusPending BundledInStatusEnum = "pending" // Has RdbCurrentBundledIn, not timeout
	BundledInStatusTimeout BundledInStatusEnum = "timeout" // Has RdbCurrentBundledIn, over 1 hour
)

type BundledInStatus struct {
	BundledInID string   `json:"bundledInID"`
	TxIds       []string `json:"txIds"`
	UploadTime  int64    `json:"uploadTime"`
}

func (c *Chainkit) moveToUploading() (int64, error) {
	// Check if RdbCurrentBundledIn exists, return failure if it does
	// Check uploading MaxCount, how many slots are available
	// Move transactions from pending to uploading, but don't exceed MaxCount
	// All operations use pipeline execution

	// 1. Check if RdbCurrentBundledIn exists - return failure if it does
	currentBundledIn, err := c.getBundledIn()
	if err != nil {
		return 0, fmt.Errorf("failed to check current bundledIn: %w", err)
	}
	if currentBundledIn != "" {
		return 0, fmt.Errorf("bundledIn already exists: %s", currentBundledIn)
	}

	// 2. Check current uploading count and calculate how many more can be added
	currentUploadingCount, err := c.uploadingCount()
	if err != nil {
		return 0, fmt.Errorf("failed to get uploading count: %w", err)
	}

	availableSlots := MaxUploadingCount - currentUploadingCount
	if availableSlots <= 0 {
		return 0, fmt.Errorf("uploading queue is full (max: %d, current: %d)", MaxUploadingCount, currentUploadingCount)
	}

	// 3. Get pending transactions to move
	pendingCount, err := c.pendingCount()
	if err != nil {
		return 0, fmt.Errorf("failed to get pending count: %w", err)
	}

	if pendingCount == 0 {
		return 0, fmt.Errorf("no pending transactions available")
	}

	// Calculate how many to move (don't exceed available slots or pending count)
	toMove := availableSlots
	if pendingCount < availableSlots {
		toMove = pendingCount
	}

	// 4. Use pipeline to execute all operations atomically
	pipe := c.redis.Pipeline()

	// Use atomic LPopCount to get and remove transactions from pending
	pendingTxids, err := c.redis.LPopCount(c.ctx, RdbPendingTxIds, int(toMove)).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, fmt.Errorf("no pending transactions available")
		}
		return 0, fmt.Errorf("failed to pop pending transactions: %w", err)
	}

	if len(pendingTxids) == 0 {
		return 0, fmt.Errorf("no pending transactions to move")
	}

	// Add popped transactions to uploading set
	if len(pendingTxids) > 0 {
		pipe.SAdd(c.ctx, RdbUploadingTxIds, pendingTxids)
	}

	// Execute pipeline
	_, err = pipe.Exec(c.ctx)
	if err != nil {
		return 0, fmt.Errorf("pipeline execution failed: %w", err)
	}

	return int64(len(pendingTxids)), nil
}

func (c *Chainkit) endUpload() error {
	// Move transactions from RdbUploadingTxIds to RdbUploadedTxIds
	// Delete RdbCurrentBundledIn
	// All operations use pipeline execution

	// First get the current bundledIn ID
	bundledInID, err := c.getBundledIn()
	if err != nil {
		return fmt.Errorf("failed to get current bundledIn: %w", err)
	}
	if bundledInID == "" {
		return fmt.Errorf("no current bundledIn to move")
	}

	// Get all uploading txids
	uploadingTxids, err := c.redis.SMembers(c.ctx, RdbUploadingTxIds).Result()
	if err != nil {
		return fmt.Errorf("failed to get uploading txids: %w", err)
	}

	// Use pipeline to execute all operations atomically
	pipe := c.redis.Pipeline()

	// 1. Move all uploading txids to uploaded
	if len(uploadingTxids) > 0 {
		pipe.SAdd(c.ctx, RdbUploadedTxIds, uploadingTxids)
	}

	// 2. Clear the uploading set (delete the key)
	pipe.Del(c.ctx, RdbUploadingTxIds)

	// 3. Delete the current bundledIn
	pipe.Del(c.ctx, RdbCurrentBundledIn)

	// Execute pipeline
	_, err = pipe.Exec(c.ctx)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	return nil
}

// addPending adds a txid to pending txids queue (FIFO)
func (c *Chainkit) addPending(txid string) error {
	return c.redis.RPush(c.ctx, RdbPendingTxIds, txid).Err()
}

// pendingCount returns the number of pending transactions in the queue
func (c *Chainkit) pendingCount() (int64, error) {
	return c.redis.LLen(c.ctx, RdbPendingTxIds).Result()
}

// uploadingCount returns the current number of uploading transactions
func (c *Chainkit) uploadingCount() (int64, error) {
	return c.redis.SCard(c.ctx, RdbUploadingTxIds).Result()
}

func (c *Chainkit) getUploading() ([]string, error) {
	return c.redis.SMembers(c.ctx, RdbUploadingTxIds).Result()
}

func (c *Chainkit) setBundledIn(bundledInID string) error {
	return c.redis.Set(c.ctx, RdbCurrentBundledIn, bundledInID, 1*time.Hour).Err()
}

// getBundledIn returns the current bundledIn ID if it exists, or empty string if it doesn't exist
func (c *Chainkit) getBundledIn() (string, error) {
	currentBundledIn, err := c.redis.Get(c.ctx, RdbCurrentBundledIn).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil // No current bundledIn, return empty string
		}
		return "", fmt.Errorf("failed to get current bundledIn: %w", err)
	}
	return currentBundledIn, nil
}

func (c *Chainkit) addUploaded(txids []string) error {
	return c.redis.SAdd(c.ctx, RdbUploadedTxIds, txids).Err()
}

// isUploaded checks if a transaction ID has already been uploaded
func (c *Chainkit) isUploaded(txid string) (bool, error) {
	return c.redis.SIsMember(c.ctx, RdbUploadedTxIds, txid).Result()
}

// isUploadedBatch checks if multiple transaction IDs have already been uploaded
// Returns a map where key is txid and value is whether it's uploaded
func (c *Chainkit) isUploadedBatch(txids []string) (map[string]bool, error) {
	if len(txids) == 0 {
		return make(map[string]bool), nil
	}

	// Use pipeline to check multiple txids efficiently
	pipe := c.redis.Pipeline()
	results := make([]*redis.BoolCmd, len(txids))

	for i, txid := range txids {
		results[i] = pipe.SIsMember(c.ctx, RdbUploadedTxIds, txid)
	}

	_, err := pipe.Exec(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check uploaded txids: %w", err)
	}

	// Build result map
	result := make(map[string]bool, len(txids))
	for i, txid := range txids {
		result[txid], err = results[i].Result()
		if err != nil {
			return nil, fmt.Errorf("failed to check txid %s: %w", txid, err)
		}
	}

	return result, nil
}
