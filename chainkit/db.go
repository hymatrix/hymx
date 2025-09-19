package chainkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	RedisUrl            = "redis://@localhost:6379/0"
	RdbPendingTxIds     = "chainkit:pending"              // Set: Pending TxId pool
	RdbCurrentBundledIn = "chainkit:current_bundledin_id" // String: current bundledIn id
	RdbBundledInStatus  = "chainkit:bundledin_status"     // Hash: chainkit:bundledin_status:<bundledInID>, BundleInStatus struct
	RdbUploadedTxIds    = "chainkit:uploaded_txids"       // Set: Uploaded TxId pool
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

// addTx add a txid to pending txids set
func (c *Chainkit) addPendingTx(txid string) error {
	return c.redis.SAdd(c.ctx, RdbPendingTxIds, txid).Err()
}

func (c *Chainkit) getPendingTxs() ([]string, error) {
	return c.redis.SMembers(c.ctx, RdbPendingTxIds).Result()
}

func (c *Chainkit) createBundledIn(bundledInID string, txids []string, uploadTime int64) error {
	// First check if RdbCurrentBundledIn is empty, if not empty return error
	currentBundledIn, err := c.redis.Get(c.ctx, RdbCurrentBundledIn).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to check current bundledIn: %w", err)
	}
	if currentBundledIn != "" {
		return ErrBundledInAlreadyExists
	}
	return c.updateBundledIn(bundledInID, txids, uploadTime)
}

func (c *Chainkit) updateBundledIn(bundledInID string, txids []string, uploadTime int64) error {
	if len(txids) == 0 {
		return ErrTxIdsEmpty
	}

	// Generate BundledInStatus struct
	status := BundledInStatus{
		BundledInID: bundledInID,
		TxIds:       txids,
		UploadTime:  uploadTime,
	}
	statusJSON, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal bundledIn status failed: %w", err)
	}

	// Put all redis operations in one pipeline
	pipe := c.redis.Pipeline()

	// If there is RdbCurrentBundledIn, delete old bundledInID first
	currentBundledIn, err := c.redis.Get(c.ctx, RdbCurrentBundledIn).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to check current bundledIn: %w", err)
	}
	if currentBundledIn != "" {
		pipe.Del(c.ctx, RdbCurrentBundledIn)
	}
	// Clear old bundledInStatus
	oldBundledInStatusKey := RdbBundledInStatus + ":" + currentBundledIn
	pipe.Del(c.ctx, oldBundledInStatusKey)

	// 1. Remove txids from RdbPendingTxIds
	if len(txids) > 0 {
		pipe.SRem(c.ctx, RdbPendingTxIds, txids)
	}

	// 2. Save BundledInStatus to RdbBundledInStatus:<bundledInID>
	bundledInStatusKey := RdbBundledInStatus + ":" + bundledInID
	pipe.Set(c.ctx, bundledInStatusKey, string(statusJSON), 0)

	// 3. Save bundledInID to RdbCurrentBundledIn
	pipe.Set(c.ctx, RdbCurrentBundledIn, bundledInID, 0)

	// Execute pipeline
	_, err = pipe.Exec(c.ctx)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	return nil
}

func (c *Chainkit) deleteBundledIn(bundledInID string) error {
	// Delete current bundledIn from RdbCurrentBundledIn
	pipe := c.redis.Pipeline()
	pipe.Del(c.ctx, RdbCurrentBundledIn)

	// Clear old bundledInStatus
	bundledInStatusKey := RdbBundledInStatus + ":" + bundledInID
	pipe.Del(c.ctx, bundledInStatusKey)

	// Execute pipeline
	_, err := pipe.Exec(c.ctx)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	return nil
}

// getBundledInStatus returns the current bundledIn status enum
func (c *Chainkit) getBundledInStatus() (BundledInStatusEnum, BundledInStatus, error) {
	currentBundledIn, err := c.redis.Get(c.ctx, RdbCurrentBundledIn).Result()
	if err != nil {
		if err == redis.Nil {
			return BundledInStatusEmpty, BundledInStatus{}, nil // No current bundledIn
		}
		return "", BundledInStatus{}, fmt.Errorf("failed to check current bundledIn: %w", err)
	}

	// If no current bundledIn, return empty
	if currentBundledIn == "" {
		return BundledInStatusEmpty, BundledInStatus{}, nil // No current bundledIn
	}

	// Get bundledIn status data
	bundledInStatusKey := RdbBundledInStatus + ":" + currentBundledIn
	statusJSON, err := c.redis.Get(c.ctx, bundledInStatusKey).Result()
	if err != nil {
		if err == redis.Nil {
			// Have currentBundledIn but no status data, may be abnormal situation, return pending
			return BundledInStatusPending, BundledInStatus{}, nil // No current bundledIn
		}
		return "", BundledInStatus{}, fmt.Errorf("failed to get bundledIn status: %w", err)
	}

	var status BundledInStatus
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		return "", BundledInStatus{}, fmt.Errorf("failed to unmarshal bundledIn status: %w", err)
	}

	// Check if timeout (over 1 hour)
	if status.UploadTime > 0 {
		currentTime := time.Now().Unix()
		if currentTime-status.UploadTime > 3600 { // 3600 seconds = 1 hour
			return BundledInStatusTimeout, status, nil // Timeout
		}
	}

	return BundledInStatusPending, status, nil // Pending
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
