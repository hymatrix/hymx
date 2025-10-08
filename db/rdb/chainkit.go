package rdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goarSchema "github.com/permadao/goar/schema"
	"github.com/redis/go-redis/v9"
)

const (
	RdbPendingTxIds     = "chainkit:pending"              // List: Pending TxId FIFO queue
	RdbUploadingTxIds   = "chainkit:uploading"            // Set: Uploading TxId pool
	RdbCurrentBundledIn = "chainkit:current_bundledin_id" // String: current bundledIn id with 1 hour expiration
	RdbUploadedTxIds    = "chainkit:uploaded_txids"       // Set: Uploaded TxId pool
	RdbMessageCache     = "chainkit:cache"                // key: pid+nonce, value message & assignment
)

const (
	// MaxUploadingCount is the maximum number of transactions allowed in uploading state
	MaxUploadingCount = 100000 // 10w
)

type Chainkit struct {
	redis *redis.Client
	ctx   context.Context
}

func NewChainkitDB(redisUrl string) *Chainkit {
	redisOpt, err := redis.ParseURL(redisUrl)
	if err != nil {
		panic(err)
	}
	redisOpt.PoolSize = 500
	redisOpt.MinIdleConns = 50
	redisOpt.MaxRetries = 3

	return &Chainkit{
		redis: redis.NewClient(redisOpt),
		ctx:   context.Background(),
	}
}

func (r *Chainkit) MoveToUploading() (int64, error) {
	// Check if RdbCurrentBundledIn exists, return failure if it does
	// Check uploading MaxCount, how many slots are available
	// Move transactions from pending to uploading, but don't exceed MaxCount
	// All operations use pipeline execution

	// 1. Check if RdbCurrentBundledIn exists - return failure if it does
	currentBundledIn, err := r.GetBundledIn()
	if err != nil {
		return 0, fmt.Errorf("failed to check current bundledIn: %w", err)
	}
	if currentBundledIn != "" {
		return 0, fmt.Errorf("bundledIn already exists: %s", currentBundledIn)
	}

	// 2. Check current uploading count and calculate how many more can be added
	currentUploadingCount, err := r.uploadingCount()
	if err != nil {
		return 0, fmt.Errorf("failed to get uploading count: %w", err)
	}

	availableSlots := MaxUploadingCount - currentUploadingCount
	if availableSlots <= 0 {
		return 0, fmt.Errorf("uploading queue is full (max: %d, current: %d)", MaxUploadingCount, currentUploadingCount)
	}

	// 3. Get pending transactions to move
	pendingCount, err := r.pendingCount()
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

	// 4. Use LPopCount to get and remove transactions from pending
	pendingTxids, err := r.redis.LPopCount(r.ctx, RdbPendingTxIds, int(toMove)).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, fmt.Errorf("no pending transactions available")
		}
		return 0, fmt.Errorf("failed to pop pending transactions: %w", err)
	}

	if len(pendingTxids) == 0 {
		return 0, fmt.Errorf("no pending transactions to move")
	}

	// 5. Use pipeline to add popped transactions to uploading set
	pipe := r.redis.Pipeline()
	pipe.SAdd(r.ctx, RdbUploadingTxIds, pendingTxids)

	// Execute pipeline
	_, err = pipe.Exec(r.ctx)
	if err != nil {
		return 0, fmt.Errorf("pipeline execution failed: %w", err)
	}

	return int64(len(pendingTxids)), nil
}

func (r *Chainkit) EndUpload() error {
	// Move transactions from RdbUploadingTxIds to RdbUploadedTxIds
	// Delete RdbCurrentBundledIn
	// All operations use pipeline execution

	// First get the current bundledIn ID
	bundledInID, err := r.GetBundledIn()
	if err != nil {
		return fmt.Errorf("failed to get current bundledIn: %w", err)
	}
	if bundledInID == "" {
		return fmt.Errorf("no current bundledIn to move")
	}

	// Get all uploading txids
	uploadingTxids, err := r.redis.SMembers(r.ctx, RdbUploadingTxIds).Result()
	if err != nil {
		return fmt.Errorf("failed to get uploading txids: %w", err)
	}

	// Use pipeline to execute all operations atomically
	pipe := r.redis.Pipeline()

	// 1. Move all uploading txids to uploaded
	if len(uploadingTxids) > 0 {
		pipe.SAdd(r.ctx, RdbUploadedTxIds, uploadingTxids)
	}

	// 2. Clear the uploading set (delete the key)
	pipe.Del(r.ctx, RdbUploadingTxIds)

	// 3. Delete the current bundledIn
	pipe.Del(r.ctx, RdbCurrentBundledIn)

	// Execute pipeline
	_, err = pipe.Exec(r.ctx)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	return nil
}

// addPending adds a txid to pending txids queue (FIFO)
func (r *Chainkit) AddPending(txid string) error {
	return r.redis.RPush(r.ctx, RdbPendingTxIds, txid).Err()
}

// pendingCount returns the number of pending transactions in the queue
func (r *Chainkit) pendingCount() (int64, error) {
	return r.redis.LLen(r.ctx, RdbPendingTxIds).Result()
}

// uploadingCount returns the current number of uploading transactions
func (r *Chainkit) uploadingCount() (int64, error) {
	return r.redis.SCard(r.ctx, RdbUploadingTxIds).Result()
}

func (r *Chainkit) GetUploading() ([]string, error) {
	return r.redis.SMembers(r.ctx, RdbUploadingTxIds).Result()
}

func (r *Chainkit) SetBundledIn(bundledInID string) error {
	return r.redis.Set(r.ctx, RdbCurrentBundledIn, bundledInID, 1*time.Hour).Err()
}

// GetBundledIn returns the current bundledIn ID if it exists, or empty string if it doesn't exist
func (r *Chainkit) GetBundledIn() (string, error) {
	currentBundledIn, err := r.redis.Get(r.ctx, RdbCurrentBundledIn).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil // No current bundledIn, return empty string
		}
		return "", fmt.Errorf("failed to get current bundledIn: %w", err)
	}
	return currentBundledIn, nil
}

func (r *Chainkit) addUploaded(txids []string) error {
	return r.redis.SAdd(r.ctx, RdbUploadedTxIds, txids).Err()
}

// isUploaded checks if a transaction ID has already been uploaded
func (r *Chainkit) isUploaded(txid string) (bool, error) {
	return r.redis.SIsMember(r.ctx, RdbUploadedTxIds, txid).Result()
}

// isUploadedBatch checks if multiple transaction IDs have already been uploaded
// Returns a map where key is txid and value is whether it's uploaded
func (r *Chainkit) IsUploadedBatch(txids []string) (map[string]bool, error) {
	if len(txids) == 0 {
		return make(map[string]bool), nil
	}

	// Use pipeline to check multiple txids efficiently
	pipe := r.redis.Pipeline()
	results := make([]*redis.BoolCmd, len(txids))

	for i, txid := range txids {
		results[i] = pipe.SIsMember(r.ctx, RdbUploadedTxIds, txid)
	}

	_, err := pipe.Exec(r.ctx)
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

func (r *Chainkit) Cache(pid string, nonce int64, msg, assignment goarSchema.BundleItem) error {
	// Create key: pid+nonce
	key := fmt.Sprintf("%s:%s:%d", RdbMessageCache, pid, nonce)
	
	// Marshal message and assignment to JSON
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	assignBytes, err := json.Marshal(assignment)
	if err != nil {
		return fmt.Errorf("failed to marshal assignment: %w", err)
	}
	
	// Use pipeline to store both message and assignment
	pipe := r.redis.Pipeline()
	pipe.HSet(r.ctx, key, "msg", msgBytes)
	pipe.HSet(r.ctx, key, "assign", assignBytes)
	
	// Execute pipeline
	_, err = pipe.Exec(r.ctx)
	if err != nil {
		return fmt.Errorf("failed to cache message and assignment: %w", err)
	}
	
	return nil
}

func (r *Chainkit) GetCache(pid string, nonce int64) (msg, assignment goarSchema.BundleItem, err error) {
	// Create key: pid+nonce
	key := fmt.Sprintf("%s:%s:%d", RdbMessageCache, pid, nonce)
	
	// Get both message and assignment from hash
	result, err := r.redis.HMGet(r.ctx, key, "msg", "assign").Result()
	if err != nil {
		if err == redis.Nil {
			return msg, assignment, fmt.Errorf("message not found for pid %s nonce %d", pid, nonce)
		}
		return msg, assignment, fmt.Errorf("failed to get message from cache: %w", err)
	}
	
	// Check if both values exist
	if len(result) != 2 {
		return msg, assignment, fmt.Errorf("invalid cache data for pid %s nonce %d", pid, nonce)
	}
	
	// Parse message
	msgStr, ok := result[0].(string)
	if !ok {
		return msg, assignment, fmt.Errorf("invalid message data for pid %s nonce %d", pid, nonce)
	}
	
	if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
		return msg, assignment, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	
	// Parse assignment
	assignStr, ok := result[1].(string)
	if !ok {
		return msg, assignment, fmt.Errorf("invalid assignment data for pid %s nonce %d", pid, nonce)
	}
	
	if err := json.Unmarshal([]byte(assignStr), &assignment); err != nil {
		return msg, assignment, fmt.Errorf("failed to unmarshal assignment: %w", err)
	}
	
	return msg, assignment, nil
}
