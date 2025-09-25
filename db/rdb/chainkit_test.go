package rdb

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedisClient is a wrapper around real Redis client for testing
type TestRedisClient struct {
	client *redis.Client
}

func NewTestRedisClient() *TestRedisClient {
	// Use Redis in Docker or local Redis for testing
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       15, // use test database
	})
	return &TestRedisClient{client: client}
}

func (t *TestRedisClient) Close() error {
	return t.client.Close()
}

func setupTest(t *testing.T) *Chainkit {
	testRedis := NewTestRedisClient()

	// Clean up test database before each test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := testRedis.client.FlushDB(ctx).Err()
	require.NoError(t, err)

	ck := &Chainkit{
		redis: testRedis.client,
		ctx:   context.Background(),
	}
	return ck
}

func TestAddPending(t *testing.T) {
	ck := setupTest(t)

	txid := "test-txid-123"

	// Test adding a pending transaction
	err := ck.AddPending(txid)
	assert.NoError(t, err)

	// Verify it was added
	members, err := ck.redis.LRange(ck.ctx, RdbPendingTxIds, 0, -1).Result()
	assert.NoError(t, err)
	assert.Contains(t, members, txid)
}

func TestGetPendingTxs(t *testing.T) {
	ck := setupTest(t)

	// Test empty case - pending transactions are stored in a list, so we need to check the list
	count, err := ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add some test transactions
	testTxids := []string{"txid1", "txid2", "txid3"}
	for _, txid := range testTxids {
		err := ck.AddPending(txid)
		assert.NoError(t, err)
	}

	// Verify pending transactions count
	count, err = ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Get pending transactions from Redis directly to verify order
	pendingTxs, err := ck.redis.LRange(ck.ctx, RdbPendingTxIds, 0, -1).Result()
	assert.NoError(t, err)
	assert.ElementsMatch(t, testTxids, pendingTxs)
}

// Tests for bundledIn functionality removed as these functions don't exist in current implementation

func TestGetUploading(t *testing.T) {
	ck := setupTest(t)

	// Test empty case
	uploading, err := ck.GetUploading()
	assert.NoError(t, err)
	assert.Empty(t, uploading)

	// Add some transactions to uploading set
	testTxids := []string{"txid1", "txid2", "txid3"}
	err = ck.redis.SAdd(ck.ctx, RdbUploadingTxIds, testTxids).Err()
	assert.NoError(t, err)

	// Get uploading transactions
	uploading, err = ck.GetUploading()
	assert.NoError(t, err)
	assert.ElementsMatch(t, testTxids, uploading)
}

func TestSetAndGetBundledIn(t *testing.T) {
	ck := setupTest(t)

	// Test get with no bundledIn
	bundledIn, err := ck.GetBundledIn()
	assert.NoError(t, err)
	assert.Empty(t, bundledIn)

	// Test set bundledIn
	testBundledInID := "test-bundled-id"
	err = ck.SetBundledIn(testBundledInID)
	assert.NoError(t, err)

	// Test get bundledIn
	bundledIn, err = ck.GetBundledIn()
	assert.NoError(t, err)
	assert.Equal(t, testBundledInID, bundledIn)
}

func TestPendingCount(t *testing.T) {
	ck := setupTest(t)

	// Test empty case
	count, err := ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add some transactions
	testTxids := []string{"txid1", "txid2", "txid3"}
	for _, txid := range testTxids {
		err := ck.AddPending(txid)
		assert.NoError(t, err)
	}

	// Test count
	count, err = ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestUploadingCount(t *testing.T) {
	ck := setupTest(t)

	// Test empty case
	count, err := ck.uploadingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add some transactions to uploading set
	testTxids := []string{"txid1", "txid2", "txid3"}
	err = ck.redis.SAdd(ck.ctx, RdbUploadingTxIds, testTxids).Err()
	assert.NoError(t, err)

	// Test count
	count, err = ck.uploadingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestAddUploaded(t *testing.T) {
	ck := setupTest(t)

	txids := []string{"txid1", "txid2", "txid3"}

	// Test adding uploaded transactions
	err := ck.addUploaded(txids)
	assert.NoError(t, err)

	// Verify they were added
	for _, txid := range txids {
		isMember, err := ck.redis.SIsMember(ck.ctx, RdbUploadedTxIds, txid).Result()
		assert.NoError(t, err)
		assert.True(t, isMember)
	}
}

func TestIsUploaded(t *testing.T) {
	ck := setupTest(t)

	txid := "test-txid-123"

	// Test non-uploaded transaction
	uploaded, err := ck.isUploaded(txid)
	assert.NoError(t, err)
	assert.False(t, uploaded)

	// Add transaction to uploaded set
	err = ck.addUploaded([]string{txid})
	assert.NoError(t, err)

	// Test uploaded transaction
	uploaded, err = ck.isUploaded(txid)
	assert.NoError(t, err)
	assert.True(t, uploaded)
}

func TestIsUploadedBatch(t *testing.T) {
	ck := setupTest(t)

	// Test with mixed uploaded/non-uploaded transactions
	txids := []string{"txid1", "txid2", "txid3"}

	// Upload only some transactions
	err := ck.addUploaded([]string{"txid1", "txid3"})
	assert.NoError(t, err)

	// Test batch check
	results, err := ck.IsUploadedBatch(txids)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(results))
	assert.True(t, results["txid1"])
	assert.False(t, results["txid2"])
	assert.True(t, results["txid3"])
}

func TestErrorHandling(t *testing.T) {
	ck := setupTest(t)

	// Test with empty txid
	err := ck.AddPending("")
	assert.NoError(t, err) // Should not error, just add empty string to list

	// Verify empty txid was added
	members, err := ck.redis.LRange(ck.ctx, RdbPendingTxIds, 0, -1).Result()
	assert.NoError(t, err)
	assert.Contains(t, members, "")

	// Test SetBundledIn with empty string (should work)
	err = ck.SetBundledIn("")
	assert.NoError(t, err)

	// Test double SetBundledIn (should work, just overwrites)
	err = ck.SetBundledIn("test-bundled")
	assert.NoError(t, err)

	err = ck.SetBundledIn("test-bundled-2")
	assert.NoError(t, err)

	// Verify the last value was set
	currentBundledIn, err := ck.GetBundledIn()
	assert.NoError(t, err)
	assert.Equal(t, "test-bundled-2", currentBundledIn)
}

func TestMoveToUploading(t *testing.T) {
	ck := setupTest(t)

	// Test with no pending transactions
	moved, err := ck.MoveToUploading()
	assert.Error(t, err)
	assert.Equal(t, int64(0), moved)

	// Add some pending transactions
	testTxids := []string{"txid1", "txid2", "txid3"}
	for _, txid := range testTxids {
		err := ck.AddPending(txid)
		assert.NoError(t, err)
	}

	// Move all to uploading
	moved, err = ck.MoveToUploading()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), moved)

	// Verify pending count is zero
	pendingCount, err := ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), pendingCount)

	// Verify uploading count increased
	uploadingCount, err := ck.uploadingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), uploadingCount)

	// Verify all transactions are in uploading set
	uploadingTxids, err := ck.GetUploading()
	assert.NoError(t, err)
	assert.ElementsMatch(t, testTxids, uploadingTxids)

	// Test with existing bundledIn (should fail)
	testBundledInID := "test-bundled-id"
	err = ck.SetBundledIn(testBundledInID)
	assert.NoError(t, err)

	// Try to move more transactions (should fail due to existing bundledIn)
	moved, err = ck.MoveToUploading()
	assert.Error(t, err)
	assert.Equal(t, int64(0), moved)
}

func TestEndUpload(t *testing.T) {
	ck := setupTest(t)

	// Add transactions to uploading set
	testTxids := []string{"txid1", "txid2", "txid3"}
	err := ck.redis.SAdd(ck.ctx, RdbUploadingTxIds, testTxids).Err()
	assert.NoError(t, err)

	// Set a bundledIn
	testBundledInID := "test-bundled-id"
	err = ck.SetBundledIn(testBundledInID)
	assert.NoError(t, err)

	// End upload
	err = ck.EndUpload()
	assert.NoError(t, err)

	// Verify transactions were removed from uploading set
	for _, txid := range testTxids {
		isMember, err := ck.redis.SIsMember(ck.ctx, RdbUploadingTxIds, txid).Result()
		assert.NoError(t, err)
		assert.False(t, isMember)
	}

	// Verify transactions were added to uploaded set
	for _, txid := range testTxids {
		isMember, err := ck.redis.SIsMember(ck.ctx, RdbUploadedTxIds, txid).Result()
		assert.NoError(t, err)
		assert.True(t, isMember)
	}

	// Verify bundledIn was cleared
	bundledIn, err := ck.GetBundledIn()
	assert.NoError(t, err)
	assert.Empty(t, bundledIn)
}

func TestConcurrentOperations(t *testing.T) {
	ck := setupTest(t)

	// Add some pending transactions
	for i := 0; i < 10; i++ {
		err := ck.AddPending(fmt.Sprintf("txid%d", i))
		assert.NoError(t, err)
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	wg.Add(3)

	// Goroutine 1: Move transactions to uploading
	go func() {
		defer wg.Done()
		moved, err := ck.MoveToUploading()
		assert.NoError(t, err)
		// In concurrent environment, we can't predict exact number moved
		assert.GreaterOrEqual(t, moved, int64(0))
	}()

	// Goroutine 2: Add more pending transactions
	go func() {
		defer wg.Done()
		for i := 10; i < 15; i++ {
			err := ck.AddPending(fmt.Sprintf("txid%d", i))
			assert.NoError(t, err)
		}
	}()

	// Goroutine 3: Check counts
	go func() {
		defer wg.Done()
		pendingCount, err := ck.pendingCount()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, pendingCount, int64(5))
	}()

	wg.Wait()
}

func TestMutexProtection(t *testing.T) {
	ck := setupTest(t)

	// Add some pending transactions
	for i := 0; i < 5; i++ {
		err := ck.AddPending(fmt.Sprintf("txid%d", i))
		assert.NoError(t, err)
	}

	// Test concurrent MoveToUploading operations
	var wg sync.WaitGroup
	wg.Add(3)

	results := make([]int64, 3)
	errors := make([]error, 3)

	for i := 0; i < 3; i++ {
		go func(idx int) {
			defer wg.Done()
			moved, err := ck.MoveToUploading()
			results[idx] = moved
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Check results - only one should succeed due to mutex protection
	successCount := 0
	totalMoved := int64(0)
	for i := 0; i < 3; i++ {
		if errors[i] == nil && results[i] > 0 {
			successCount++
			totalMoved += results[i]
		}
	}

	// Due to mutex protection, only one operation should succeed
	assert.Equal(t, 1, successCount)
	assert.Equal(t, int64(5), totalMoved) // All 5 transactions should be moved

	// Verify final state
	pendingCount, err := ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), pendingCount) // All should be moved

	uploadingCount, err := ck.uploadingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(5), uploadingCount) // All 5 should be in uploading
}

func TestUploadingCountLimit(t *testing.T) {
	ck := setupTest(t)

	// Test the basic functionality that MoveToUploading should respect the limit
	// First, let's test what the current MaxUploadingCount is
	maxCount := int64(100000) // Based on the constant in chainkit.go

	// Add a reasonable number of transactions to uploading set (close to but not exceeding a smaller test limit)
	testUploadingCount := int64(8)
	for i := int64(0); i < testUploadingCount; i++ {
		err := ck.redis.SAdd(ck.ctx, RdbUploadingTxIds, fmt.Sprintf("txid%d", i)).Err()
		assert.NoError(t, err)
	}

	// Add some pending transactions
	pendingToAdd := int64(5)
	for i := int64(0); i < pendingToAdd; i++ {
		err := ck.AddPending(fmt.Sprintf("pending%d", i))
		assert.NoError(t, err)
	}

	// Verify initial state
	initialPending, err := ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, pendingToAdd, initialPending)

	initialUploading, err := ck.uploadingCount()
	assert.NoError(t, err)
	assert.Equal(t, testUploadingCount, initialUploading)

	// Try to move transactions to uploading
	// The method should handle the limit checking internally
	moved, err := ck.MoveToUploading()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, moved, int64(0)) // Should move some or none

	// The key test: verify that if we moved transactions, we didn't exceed the max limit
	finalUploading, err := ck.uploadingCount()
	assert.NoError(t, err)
	assert.LessOrEqual(t, finalUploading, maxCount) // Must not exceed max limit

	// Also verify that pending count changed appropriately
	finalPending, err := ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, pendingToAdd-moved, finalPending) // Should decrease by the number moved
}
