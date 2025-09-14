package chainkit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hymatrix/hymx/chainkit/schema"
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

	// First add to upload set
	for _, txid := range testTxIDs {
		err := suite.chainkit.addToUploads(txid)
		assert.NoError(suite.T(), err)
	}

	// Verify initial state
	count, err := suite.chainkit.getUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(3), count)

	// Execute move operation (moveToPending moves all uploads to pending with parentTxID "0")
	err = suite.chainkit.moveToPending()
	assert.NoError(suite.T(), err)

	// Verify removal from upload set
	count, err = suite.chainkit.getUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), count)

	// Verify addition to pending set (with default parent "0")
	childTxIDs, err := suite.chainkit.getPendingsByParentID(schema.ZeroParentID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs, 3)
	for _, txid := range testTxIDs {
		assert.Contains(suite.T(), childTxIDs, txid)
	}
}

// TestGetPendings test getting all pending parent transaction IDs
func (suite *DBTestSuite) TestGetPendings() {
	// Test empty state
	parentTxIDs, err := suite.chainkit.getPendingParentIDs()
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), parentTxIDs)

	// Add test data
	testParents := []string{"parent1", "parent2", "parent3"}
	for _, parent := range testParents {
		testTxIDs := []string{"child1_" + parent, "child2_" + parent}
		err := suite.chainkit.updateParentId(parent, testTxIDs)
		assert.NoError(suite.T(), err)
	}

	// Test getting all parent transaction IDs
	parentTxIDs, err = suite.chainkit.getPendingParentIDs()
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
	childTxIDs, err := suite.chainkit.getPendingsByParentID("nonexistent")
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), childTxIDs)

	// Add test data
	err = suite.chainkit.updateParentId(parentTxID, testTxIDs)
	assert.NoError(suite.T(), err)

	// Test getting sub transaction IDs
	childTxIDs, err = suite.chainkit.getPendingsByParentID(parentTxID)
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

	// Add test data
	err := suite.chainkit.updateParentId(parentTxID, testTxIDs)
	assert.NoError(suite.T(), err)

	// Verify data exists
	childTxIDs, err := suite.chainkit.getPendingsByParentID(parentTxID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs, 3)

	// Execute delete operation
	err = suite.chainkit.removePending(parentTxID)
	assert.NoError(suite.T(), err)

	// Verify deletion success
	childTxIDs, err = suite.chainkit.getPendingsByParentID(parentTxID)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), childTxIDs)

	// Verify removal from parent transaction list
	parentTxIDs, err := suite.chainkit.getPendingParentIDs()
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

	// 3. Move to pending set in batches using updateParentId
	err = suite.chainkit.updateParentId(parentTxID1, testTxIDs[:3])
	assert.NoError(suite.T(), err)

	err = suite.chainkit.updateParentId(parentTxID2, testTxIDs[3:])
	assert.NoError(suite.T(), err)

	// 4. Verify pending set status
	parentTxIDs, err := suite.chainkit.getPendingParentIDs()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), parentTxIDs, 2)
	assert.Contains(suite.T(), parentTxIDs, parentTxID1)
	assert.Contains(suite.T(), parentTxIDs, parentTxID2)

	// 5. Verify sub transaction grouping
	childTxIDs1, err := suite.chainkit.getPendingsByParentID(parentTxID1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs1, 3)

	childTxIDs2, err := suite.chainkit.getPendingsByParentID(parentTxID2)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs2, 2)

	// 6. Remove one parent transaction
	err = suite.chainkit.removePending(parentTxID1)
	assert.NoError(suite.T(), err)

	// 7. Verify final state
	parentTxIDs, err = suite.chainkit.getPendingParentIDs()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), parentTxIDs, 1)
	assert.Contains(suite.T(), parentTxIDs, parentTxID2)
}

// TestFindParentByTxid test finding parent transaction by sub transaction ID
func (suite *DBTestSuite) TestFindParentByTxid() {
	// Test with non-existent txid
	parentID, err := suite.chainkit.findParentByTxid("nonexistent_tx")
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), parentID)

	// Prepare test data
	parentTxID1 := "parent_tx_001"
	parentTxID2 := "parent_tx_002"
	testTxIDs1 := []string{"child1", "child2"}
	testTxIDs2 := []string{"child3", "child4"}

	// Add test data to uploads first, then move to pending
	for _, txid := range testTxIDs1 {
		err := suite.chainkit.addToUploads(txid)
		assert.NoError(suite.T(), err)
	}
	err = suite.chainkit.updateParentId(parentTxID1, testTxIDs1)
	assert.NoError(suite.T(), err)

	for _, txid := range testTxIDs2 {
		err := suite.chainkit.addToUploads(txid)
		assert.NoError(suite.T(), err)
	}
	err = suite.chainkit.updateParentId(parentTxID2, testTxIDs2)
	assert.NoError(suite.T(), err)

	// Test finding existing parent IDs
	parentID, err = suite.chainkit.findParentByTxid("child1")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), parentTxID1, parentID)

	parentID, err = suite.chainkit.findParentByTxid("child3")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), parentTxID2, parentID)

	// Test with non-existent txid after adding data
	parentID, err = suite.chainkit.findParentByTxid("nonexistent_child")
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), parentID)
}

// TestUpdateParentId test updating parent transaction ID
func (suite *DBTestSuite) TestUpdateParentId() {
	// Prepare initial data
	oldParentTxID := "old_parent_123"
	newParentTxID := "new_parent_456"
	testTxIDs := []string{"tx1", "tx2", "tx3"}

	// First add to old parent
	err := suite.chainkit.updateParentId(oldParentTxID, testTxIDs)
	assert.NoError(suite.T(), err)

	// Verify initial state
	childTxIDs, err := suite.chainkit.getPendingsByParentID(oldParentTxID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs, 3)

	// Update parent ID
	err = suite.chainkit.updateParentId(newParentTxID, testTxIDs)
	assert.NoError(suite.T(), err)

	// Verify old parent is cleaned up
	childTxIDs, err = suite.chainkit.getPendingsByParentID(oldParentTxID)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), childTxIDs)

	// Verify new parent has the transactions
	childTxIDs, err = suite.chainkit.getPendingsByParentID(newParentTxID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs, 3)
	for _, txid := range testTxIDs {
		assert.Contains(suite.T(), childTxIDs, txid)
	}

	// Verify findParentByTxid returns new parent
	for _, txid := range testTxIDs {
		parentID, err := suite.chainkit.findParentByTxid(txid)
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), newParentTxID, parentID)
	}
}

// TestUpdateParentIdWithMultipleOldParents test updating parent ID with transactions from multiple old parents
func (suite *DBTestSuite) TestUpdateParentIdWithMultipleOldParents() {
	// Prepare data from multiple old parents
	oldParent1 := "old_parent_1"
	oldParent2 := "old_parent_2"
	newParent := "new_parent"
	txidsFromParent1 := []string{"tx1", "tx2"}
	txidsFromParent2 := []string{"tx3", "tx4"}
	allTxids := append(txidsFromParent1, txidsFromParent2...)

	// Add to different old parents
	err := suite.chainkit.updateParentId(oldParent1, txidsFromParent1)
	assert.NoError(suite.T(), err)
	err = suite.chainkit.updateParentId(oldParent2, txidsFromParent2)
	assert.NoError(suite.T(), err)

	// Verify initial state
	childTxIDs1, err := suite.chainkit.getPendingsByParentID(oldParent1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs1, 2)

	childTxIDs2, err := suite.chainkit.getPendingsByParentID(oldParent2)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs2, 2)

	// Update all transactions to new parent
	err = suite.chainkit.updateParentId(newParent, allTxids)
	assert.NoError(suite.T(), err)

	// Verify both old parents are cleaned up
	childTxIDs1, err = suite.chainkit.getPendingsByParentID(oldParent1)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), childTxIDs1)

	childTxIDs2, err = suite.chainkit.getPendingsByParentID(oldParent2)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), childTxIDs2)

	// Verify new parent has all transactions
	childTxIDs, err := suite.chainkit.getPendingsByParentID(newParent)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs, 4)
	for _, txid := range allTxids {
		assert.Contains(suite.T(), childTxIDs, txid)
	}
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

	// Test deleting non-existent parent transaction
	err = suite.chainkit.removePending("nonexistent_parent")
	assert.NoError(suite.T(), err)

	// Test updateParentId with empty txids
	err = suite.chainkit.updateParentId("new_parent", []string{})
	assert.NoError(suite.T(), err)

	// Test updateParentId with non-existent txids
	err = suite.chainkit.updateParentId("new_parent", []string{"nonexistent1", "nonexistent2"})
	assert.NoError(suite.T(), err)
}

// TestUploadedTxids tests uploaded txid tracking functionality
func (suite *DBTestSuite) TestUploadedTxids() {
	// Test adding uploaded txids
	txid1 := "uploaded-txid-1"
	txid2 := "uploaded-txid-2"

	// Initially should not be uploaded
	uploaded, err := suite.chainkit.isUploaded(txid1)
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), uploaded)

	// Add to uploaded set
	err = suite.chainkit.addUploaded([]string{txid1})
	assert.NoError(suite.T(), err)
	err = suite.chainkit.addUploaded([]string{txid2})
	assert.NoError(suite.T(), err)

	// Check if uploaded
	uploaded, err = suite.chainkit.isUploaded(txid1)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), uploaded)

	uploaded, err = suite.chainkit.isUploaded(txid2)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), uploaded)

	// Test duplicate addition (should not increase count)
	err = suite.chainkit.addUploaded([]string{txid1})
	assert.NoError(suite.T(), err)

	// Since we only have isUploaded and addUploaded methods, we'll test basic functionality
	uploaded, err = suite.chainkit.isUploaded(txid1)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), uploaded)

	uploaded, err = suite.chainkit.isUploaded(txid2)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), uploaded)
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
		txid := fmt.Sprintf("benchmark_tx_%d", i)
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
		txid := fmt.Sprintf("benchmark_tx_%d", i)
		chainkit.addToUploads(txid)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chainkit.getUploads()
	}
}
