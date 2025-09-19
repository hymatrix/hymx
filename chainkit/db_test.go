package chainkit

import (
	"context"
	"encoding/json"
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

func TestAddPendingTx(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()
	
	txid := "test-txid-123"
	
	// Test adding a pending transaction
	err := ck.addPendingTx(txid)
	assert.NoError(t, err)
	
	// Verify it was added
	members, err := ck.redis.SMembers(ck.ctx, RdbPendingTxIds).Result()
	assert.NoError(t, err)
	assert.Contains(t, members, txid)
}

func TestGetPendingTxs(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()
	
	// Test empty case
	txs, err := ck.getPendingTxs()
	assert.NoError(t, err)
	assert.Empty(t, txs)
	
	// Add some test transactions
	testTxids := []string{"txid1", "txid2", "txid3"}
	for _, txid := range testTxids {
		err := ck.addPendingTx(txid)
		assert.NoError(t, err)
	}
	
	// Get pending transactions
	txs, err = ck.getPendingTxs()
	assert.NoError(t, err)
	assert.ElementsMatch(t, testTxids, txs)
}

func TestCreateBundledIn(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()
	
	bundledInID := "test-bundled-id"
	txids := []string{"txid1", "txid2"}
	uploadTime := time.Now().Unix()
	
	// Test creating a bundled in entry
	err := ck.createBundledIn(bundledInID, txids, uploadTime)
	assert.NoError(t, err)
	
	// Verify it was created
	result, err := ck.redis.Get(ck.ctx, RdbCurrentBundledIn).Result()
	assert.NoError(t, err)
	assert.Equal(t, bundledInID, result)
	
	// Verify status was created
	statusKey := RdbBundledInStatus + ":" + bundledInID
	statusJSON, err := ck.redis.Get(ck.ctx, statusKey).Result()
	assert.NoError(t, err)
	
	var status BundledInStatus
	err = json.Unmarshal([]byte(statusJSON), &status)
	assert.NoError(t, err)
	assert.Equal(t, bundledInID, status.BundledInID)
	assert.Equal(t, txids, status.TxIds)
	assert.Equal(t, uploadTime, status.UploadTime)
}

func TestUpdateBundledIn(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()
	
	bundledInID := "test-bundled-id"
	initialTxids := []string{"txid1", "txid2"}
	updatedTxids := []string{"txid3", "txid4"}
	
	// Create initial entry
	err := ck.createBundledIn(bundledInID, initialTxids, time.Now().Unix())
	assert.NoError(t, err)
	
	// Update the bundled in with new txids
	err = ck.updateBundledIn(bundledInID, updatedTxids, time.Now().Unix())
	assert.NoError(t, err)
	
	// Verify it was updated
	statusKey := RdbBundledInStatus + ":" + bundledInID
	statusJSON, err := ck.redis.Get(ck.ctx, statusKey).Result()
	assert.NoError(t, err)
	
	var status BundledInStatus
	err = json.Unmarshal([]byte(statusJSON), &status)
	assert.NoError(t, err)
	assert.Equal(t, updatedTxids, status.TxIds)
}

func TestDeleteBundledIn(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()
	
	bundledInID := "test-bundled-id"
	txids := []string{"txid1", "txid2"}
	
	// Create initial entry
	err := ck.createBundledIn(bundledInID, txids, time.Now().Unix())
	assert.NoError(t, err)
	
	// Delete the entry
	err = ck.deleteBundledIn(bundledInID)
	assert.NoError(t, err)
	
	// Verify it was deleted
	result, err := ck.redis.Get(ck.ctx, RdbCurrentBundledIn).Result()
	assert.Equal(t, redis.Nil, err)
	assert.Empty(t, result)
	
	statusKey := RdbBundledInStatus + ":" + bundledInID
	result, err = ck.redis.Get(ck.ctx, statusKey).Result()
	assert.Equal(t, redis.Nil, err)
	assert.Empty(t, result)
}

func TestGetBundledInStatus(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()
	
	// Test empty status
	statusEnum, status, err := ck.getBundledInStatus()
	assert.NoError(t, err)
	assert.Equal(t, BundledInStatusEmpty, statusEnum)
	assert.Empty(t, status.BundledInID)
	
	// Create entry
	bundledInID := "test-bundled-id"
	txids := []string{"txid1", "txid2"}
	err = ck.createBundledIn(bundledInID, txids, time.Now().Unix())
	assert.NoError(t, err)
	
	// Get status
	statusEnum, status, err = ck.getBundledInStatus()
	assert.NoError(t, err)
	assert.Equal(t, BundledInStatusPending, statusEnum)
	assert.Equal(t, bundledInID, status.BundledInID)
	assert.Equal(t, txids, status.TxIds)
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
	err := ck.addPendingTx("")
	assert.NoError(t, err) // Should not error, just add empty string to set
	
	// Verify empty txid was added
	isMember, err := ck.redis.SIsMember(ck.ctx, RdbPendingTxIds, "").Result()
	assert.NoError(t, err)
	assert.True(t, isMember)
	
	// Test createBundledIn with empty txids
	err = ck.createBundledIn("test-bundled", []string{}, time.Now().Unix())
	assert.Equal(t, ErrTxIdsEmpty, err)
	
	// Test double createBundledIn
	err = ck.createBundledIn("test-bundled", []string{"txid1"}, time.Now().Unix())
	assert.NoError(t, err)
	
	err = ck.createBundledIn("test-bundled", []string{"txid2"}, time.Now().Unix())
	assert.Equal(t, ErrBundledInAlreadyExists, err)
}

func TestConcurrentOperations(t *testing.T) {
	ck, testRedis := setupTest(t)
	defer testRedis.Close()
	
	// Test concurrent adds
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			txid := "txid-" + string(rune('0'+i))
			err := ck.addPendingTx(txid)
			assert.NoError(t, err)
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Verify all transactions were added
	members, err := ck.redis.SMembers(ck.ctx, RdbPendingTxIds).Result()
	assert.NoError(t, err)
	assert.Len(t, members, 10)
}