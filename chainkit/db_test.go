package chainkit

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// DBTestSuite database test suite
type DBTestSuite struct {
	suite.Suite
	chainkit *Chainkit
	rdb      *redis.Client
}

// SetupSuite test suite initialization
func (suite *DBTestSuite) SetupSuite() {
	redisOpt, err := redis.ParseURL("redis://@localhost:6379/15")
	if err != nil {
		panic(err)
	}
	redisOpt.PoolSize = 500
	redisOpt.MinIdleConns = 50
	redisOpt.MaxRetries = 3

	suite.rdb = redis.NewClient(redisOpt)

	suite.chainkit = &Chainkit{
		redis: suite.rdb,
		ctx:   context.Background(),
	}

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = suite.rdb.Ping(ctx).Result()
	if err != nil {
		suite.T().Skip("Redis server not available, skipping tests")
	}
}

// TearDownSuite test suite cleanup
func (suite *DBTestSuite) TearDownSuite() {
	if suite.rdb != nil {
		suite.rdb.Close()
	}
}

// SetupTest preparation work before each test
func (suite *DBTestSuite) SetupTest() {
	// Clean up test data
	suite.rdb.FlushDB(context.Background())
}

// TestGetUploads test getting upload set
func (suite *DBTestSuite) TestGetUploads() {
	// Initial state should be empty
	members, err := suite.chainkit.getUploads()
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), members)

	// Add test data
	testTxIDs := []string{"tx1", "tx2", "tx3"}
	for _, txid := range testTxIDs {
		err := suite.chainkit.addToUploads(txid)
		assert.NoError(suite.T(), err)
	}

	// Test getting all members
	members, err = suite.chainkit.getUploads()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), members, 3)
	for _, txid := range testTxIDs {
		assert.Contains(suite.T(), members, txid)
	}
}

// TestGetUploadsCount test getting upload set count
func (suite *DBTestSuite) TestGetUploadsCount() {
	// Initial state should be 0
	count, err := suite.chainkit.getUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), count)

	// Add test data
	testTxIDs := []string{"tx1", "tx2", "tx3"}
	for _, txid := range testTxIDs {
		err := suite.chainkit.addToUploads(txid)
		assert.NoError(suite.T(), err)
	}

	// Test count
	count, err = suite.chainkit.getUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(3), count)
}

// TestAddToUploads test adding to upload set
func (suite *DBTestSuite) TestAddToUploads() {
	testTxID := "test_tx_123"

	// Test adding
	err := suite.chainkit.addToUploads(testTxID)
	assert.NoError(suite.T(), err)

	// Verify addition success
	members, err := suite.chainkit.getUploads()
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), members, testTxID)

	// Test duplicate addition (Set feature, no duplicates)
	err = suite.chainkit.addToUploads(testTxID)
	assert.NoError(suite.T(), err)

	count, err := suite.chainkit.getUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count)
}

// TestMoveToPending test moving to pending set
func (suite *DBTestSuite) TestMoveToPending() {
	// Prepare test data
	testTxIDs := []string{"tx1", "tx2", "tx3"}
	parentTxID := "parent_tx_123"

	// First add to upload set
	for _, txid := range testTxIDs {
		err := suite.chainkit.addToUploads(txid)
		assert.NoError(suite.T(), err)
	}

	// Verify initial state
	count, err := suite.chainkit.getUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(3), count)

	// Execute move operation
	err = suite.chainkit.moveToPending(parentTxID, testTxIDs)
	assert.NoError(suite.T(), err)

	// Verify removal from upload set
	count, err = suite.chainkit.getUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), count)

	// Verify addition to pending set
	childTxIDs, err := suite.chainkit.getPendingSub(parentTxID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs, 3)
	for _, txid := range testTxIDs {
		assert.Contains(suite.T(), childTxIDs, txid)
	}
}

// TestGetPendings test getting all pending parent transaction IDs
func (suite *DBTestSuite) TestGetPendings() {
	// Test empty state
	parentTxIDs, err := suite.chainkit.getPendings()
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), parentTxIDs)

	// 添加测试数据
	testParents := []string{"parent1", "parent2", "parent3"}
	for _, parent := range testParents {
		testTxIDs := []string{"child1_" + parent, "child2_" + parent}
		err := suite.chainkit.moveToPending(parent, testTxIDs)
		assert.NoError(suite.T(), err)
	}

	// Test getting all parent transaction IDs
	parentTxIDs, err = suite.chainkit.getPendings()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), parentTxIDs, 3)
	for _, parent := range testParents {
		assert.Contains(suite.T(), parentTxIDs, parent)
	}
}

// TestGetPendingSub test getting sub transaction IDs
func (suite *DBTestSuite) TestGetPendingSub() {
	parentTxID := "parent_tx_456"
	testTxIDs := []string{"child1", "child2", "child3"}

	// Test non-existent parent transaction
	childTxIDs, err := suite.chainkit.getPendingSub("nonexistent")
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), childTxIDs)

	// 添加测试数据
	err = suite.chainkit.moveToPending(parentTxID, testTxIDs)
	assert.NoError(suite.T(), err)

	// Test getting sub transaction IDs
	childTxIDs, err = suite.chainkit.getPendingSub(parentTxID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs, 3)
	for _, txid := range testTxIDs {
		assert.Contains(suite.T(), childTxIDs, txid)
	}
}

// TestRemovePending test removing pending transactions
func (suite *DBTestSuite) TestRemovePending() {
	parentTxID := "parent_tx_789"
	testTxIDs := []string{"child1", "child2", "child3"}

	// 添加测试数据
	err := suite.chainkit.moveToPending(parentTxID, testTxIDs)
	assert.NoError(suite.T(), err)

	// Verify data exists
	childTxIDs, err := suite.chainkit.getPendingSub(parentTxID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs, 3)

	// Execute delete operation
	err = suite.chainkit.removePending(parentTxID)
	assert.NoError(suite.T(), err)

	// Verify deletion success
	childTxIDs, err = suite.chainkit.getPendingSub(parentTxID)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), childTxIDs)

	// Verify removal from parent transaction list
	parentTxIDs, err := suite.chainkit.getPendings()
	assert.NoError(suite.T(), err)
	assert.NotContains(suite.T(), parentTxIDs, parentTxID)
}

// TestComplexWorkflow test complex workflow
func (suite *DBTestSuite) TestComplexWorkflow() {
	// Simulate complete workflow
	testTxIDs := []string{"tx1", "tx2", "tx3", "tx4", "tx5"}
	parentTxID1 := "parent1"
	parentTxID2 := "parent2"

	// 1. Add transactions to upload set
	for _, txid := range testTxIDs {
		err := suite.chainkit.addToUploads(txid)
		assert.NoError(suite.T(), err)
	}

	// 2. Verify upload set status
	count, err := suite.chainkit.getUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(5), count)

	// 3. Move to pending set in batches
	err = suite.chainkit.moveToPending(parentTxID1, testTxIDs[:3])
	assert.NoError(suite.T(), err)

	err = suite.chainkit.moveToPending(parentTxID2, testTxIDs[3:])
	assert.NoError(suite.T(), err)

	// 4. Verify upload set is cleared
	count, err = suite.chainkit.getUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), count)

	// 5. Verify pending set status
	parentTxIDs, err := suite.chainkit.getPendings()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), parentTxIDs, 2)
	assert.Contains(suite.T(), parentTxIDs, parentTxID1)
	assert.Contains(suite.T(), parentTxIDs, parentTxID2)

	// 6. Verify sub transaction grouping
	childTxIDs1, err := suite.chainkit.getPendingSub(parentTxID1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs1, 3)

	childTxIDs2, err := suite.chainkit.getPendingSub(parentTxID2)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs2, 2)

	// 7. Remove one parent transaction
	err = suite.chainkit.removePending(parentTxID1)
	assert.NoError(suite.T(), err)

	// 8. Verify final state
	parentTxIDs, err = suite.chainkit.getPendings()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), parentTxIDs, 1)
	assert.Contains(suite.T(), parentTxIDs, parentTxID2)
}

// TestEdgeCases test edge cases
func (suite *DBTestSuite) TestEdgeCases() {
	// Test empty string
	err := suite.chainkit.addToUploads("")
	assert.NoError(suite.T(), err)

	// Test special characters
	specialTxID := "tx:with:colons"
	err = suite.chainkit.addToUploads(specialTxID)
	assert.NoError(suite.T(), err)

	members, err := suite.chainkit.getUploads()
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), members, specialTxID)

	// Test moving empty array
	err = suite.chainkit.moveToPending("empty_parent", []string{})
	assert.NoError(suite.T(), err)

	// Test deleting non-existent parent transaction
	err = suite.chainkit.removePending("nonexistent_parent")
	assert.NoError(suite.T(), err)
}

// TestDBTestSuite run test suite
func TestDBTestSuite(t *testing.T) {
	suite.Run(t, new(DBTestSuite))
}

// Benchmark tests

// BenchmarkAddToUploads benchmark test for add operations
func BenchmarkAddToUploads(b *testing.B) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer rdb.Close()

	chainkit := &Chainkit{
		redis: rdb,
		ctx:   context.Background(),
	}

	// Clean up test data
	rdb.FlushDB(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txid := "benchmark_tx_" + string(rune(i))
		chainkit.addToUploads(txid)
	}
}

// BenchmarkGetUploads benchmark test for get operations
func BenchmarkGetUploads(b *testing.B) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer rdb.Close()

	chainkit := &Chainkit{
		redis: rdb,
		ctx:   context.Background(),
	}

	// Prepare test data
	rdb.FlushDB(context.Background())
	for i := 0; i < 1000; i++ {
		txid := "benchmark_tx_" + string(rune(i))
		chainkit.addToUploads(txid)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chainkit.getUploads()
	}
}
