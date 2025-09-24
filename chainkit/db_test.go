package chainkit

import (
	"context"
	"fmt"
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

func setupTest(t *testing.T) (*Chainkit, *TestRedisClient) {
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
	return ck, testRedis
}

func TestAddPending(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	txid := "test-txid-123"

	// Test adding a pending transaction
	err := ck.addPending(txid)
	assert.NoError(t, err)

	// Verify it was added
	members, err := ck.redis.LRange(ck.ctx, RdbPendingTxIds, 0, -1).Result()
	assert.NoError(t, err)
	assert.Contains(t, members, txid)
}

func TestGetPendingTxs(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	// Test empty case - pending transactions are stored in a list, so we need to check the list
	count, err := ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add some test transactions
	testTxids := []string{"txid1", "txid2", "txid3"}
	for _, txid := range testTxids {
		err := ck.addPending(txid)
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
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	// Test empty case
	uploading, err := ck.getUploading()
	assert.NoError(t, err)
	assert.Empty(t, uploading)

	// Add some transactions to uploading set
	testTxids := []string{"txid1", "txid2", "txid3"}
	err = ck.redis.SAdd(ck.ctx, RdbUploadingTxIds, testTxids).Err()
	assert.NoError(t, err)

	// Get uploading transactions
	uploading, err = ck.getUploading()
	assert.NoError(t, err)
	assert.ElementsMatch(t, testTxids, uploading)
}

func TestSetAndGetBundledIn(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	// Test get with no bundledIn
	bundledIn, err := ck.getBundledIn()
	assert.NoError(t, err)
	assert.Empty(t, bundledIn)

	// Test set bundledIn
	testBundledInID := "test-bundled-id"
	err = ck.setBundledIn(testBundledInID)
	assert.NoError(t, err)

	// Test get bundledIn
	bundledIn, err = ck.getBundledIn()
	assert.NoError(t, err)
	assert.Equal(t, testBundledInID, bundledIn)
}

func TestPendingCount(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	// Test empty case
	count, err := ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add some transactions
	testTxids := []string{"txid1", "txid2", "txid3"}
	for _, txid := range testTxids {
		err := ck.addPending(txid)
		assert.NoError(t, err)
	}

	// Test count
	count, err = ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestUploadingCount(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

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
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

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
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

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
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	// Test with mixed uploaded/non-uploaded transactions
	txids := []string{"txid1", "txid2", "txid3"}

	// Upload only some transactions
	err := ck.addUploaded([]string{"txid1", "txid3"})
	assert.NoError(t, err)

	// Test batch check
	results, err := ck.isUploadedBatch(txids)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(results))
	assert.True(t, results["txid1"])
	assert.False(t, results["txid2"])
	assert.True(t, results["txid3"])
}

func TestErrorHandling(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	// Test with empty txid
	err := ck.addPending("")
	assert.NoError(t, err) // Should not error, just add empty string to list

	// Verify empty txid was added
	members, err := ck.redis.LRange(ck.ctx, RdbPendingTxIds, 0, -1).Result()
	assert.NoError(t, err)
	assert.Contains(t, members, "")

	// Test setBundledIn with empty string (should work)
	err = ck.setBundledIn("")
	assert.NoError(t, err)

	// Test double setBundledIn (should work, just overwrites)
	err = ck.setBundledIn("test-bundled")
	assert.NoError(t, err)

	err = ck.setBundledIn("test-bundled-2")
	assert.NoError(t, err)

	// Verify the last value was set
	currentBundledIn, err := ck.getBundledIn()
	assert.NoError(t, err)
	assert.Equal(t, "test-bundled-2", currentBundledIn)
}

func TestMoveToUploading(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	// Test with no pending transactions
	moved, err := ck.moveToUploading()
	assert.Error(t, err)
	assert.Equal(t, int64(0), moved)

	// Add some pending transactions
	testTxids := []string{"txid1", "txid2", "txid3"}
	for _, txid := range testTxids {
		err := ck.addPending(txid)
		assert.NoError(t, err)
	}

	// Test moving to uploading
	moved, err = ck.moveToUploading()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), moved)

	// Verify pending queue is empty
	pendingCount, err := ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), pendingCount)

	// Verify uploading set has the transactions
	uploadingTxids, err := ck.getUploading()
	assert.NoError(t, err)
	assert.ElementsMatch(t, testTxids, uploadingTxids)

	// Test with existing bundledIn (should fail)
	err = ck.setBundledIn("existing-bundled")
	assert.NoError(t, err)

	moved, err = ck.moveToUploading()
	assert.Error(t, err)
	assert.Equal(t, int64(0), moved)
}

func TestEndUpload(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	// Test with no current bundledIn (should fail)
	err := ck.endUpload()
	assert.Error(t, err)

	// Set up test data
	bundledInID := "test-bundled-id"
	testTxids := []string{"txid1", "txid2", "txid3"}

	// Add transactions to uploading set
	err = ck.redis.SAdd(ck.ctx, RdbUploadingTxIds, testTxids).Err()
	assert.NoError(t, err)

	// Set current bundledIn
	err = ck.setBundledIn(bundledInID)
	assert.NoError(t, err)

	// Test endUpload
	err = ck.endUpload()
	assert.NoError(t, err)

	// Verify uploading set is empty
	uploadingCount, err := ck.uploadingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), uploadingCount)

	// Verify transactions are in uploaded set
	for _, txid := range testTxids {
		uploaded, err := ck.isUploaded(txid)
		assert.NoError(t, err)
		assert.True(t, uploaded)
	}

	// Verify current bundledIn is deleted
	currentBundledIn, err := ck.getBundledIn()
	assert.NoError(t, err)
	assert.Empty(t, currentBundledIn)
}

func TestConcurrentOperations(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	// Test concurrent adds
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			txid := "txid-" + string(rune('0'+i))
			err := ck.addPending(txid)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all transactions were added
	members, err := ck.redis.LRange(ck.ctx, RdbPendingTxIds, 0, -1).Result()
	assert.NoError(t, err)
	assert.Len(t, members, 10)
}

func TestMutexProtection(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	// Test concurrent moveToUploading operations
	// Add some pending transactions first
	for i := 0; i < 5; i++ {
		err := ck.addPending(fmt.Sprintf("txid%d", i))
		assert.NoError(t, err)
	}

	// Try concurrent moveToUploading operations
	done := make(chan bool, 3)
	results := make([]int64, 3)
	errors := make([]error, 3)

	for i := 0; i < 3; i++ {
		go func(i int) {
			results[i], errors[i] = ck.moveToUploading()
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Only one should succeed due to mutex protection
	successCount := 0
	for i := 0; i < 3; i++ {
		if errors[i] == nil && results[i] > 0 {
			successCount++
		}
	}

	// Due to mutex, only one operation should succeed
	assert.Equal(t, 1, successCount, "Only one moveToUploading should succeed due to mutex protection")
}

func TestUploadingCountLimit(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()

	// Add more transactions than the limit
	for i := 0; i < MaxUploadingCount+10; i++ {
		err := ck.addPending(fmt.Sprintf("txid%d", i))
		assert.NoError(t, err)
	}

	// First moveToUploading should move MaxUploadingCount transactions
	moved, err := ck.moveToUploading()
	assert.NoError(t, err)
	assert.Equal(t, int64(MaxUploadingCount), moved)

	// Second moveToUploading should fail due to existing bundledIn
	moved, err = ck.moveToUploading()
	assert.Error(t, err)
	assert.Equal(t, int64(0), moved)

	pendingCount, err := ck.pendingCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(10), pendingCount)
}
