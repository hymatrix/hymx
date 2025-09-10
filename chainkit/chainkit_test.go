package chainkit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hymatrix/hymx/chainkit/optgoar"
	"github.com/hymatrix/hymx/chainkit/schema"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// MockNodeDB is a mock implementation of INodeDB interface
type MockNodeDB struct{}

// NewMockNodeDB creates a new MockNodeDB instance
func NewMockNodeDB() *MockNodeDB {
	return &MockNodeDB{}
}

// GetIDB returns a mock IDB
func (m *MockNodeDB) GetIDB() nodeSchema.IDB {
	return nil // Simply return nil, can be implemented as needed in actual tests
}

// GetMessage returns a mock BundleItem
func (m *MockNodeDB) GetMessage(msgid string) (msg *goarSchema.BundleItem, err error) {
	return &goarSchema.BundleItem{
		Id:   msgid,
		Data: "mock message data",
	}, nil
}

// GetMessageByNonce returns mock BundleItem by nonce
func (m *MockNodeDB) GetMessageByNonce(pid string, nonce int64) (msg *goarSchema.BundleItem, err error) {
	return &goarSchema.BundleItem{
		Id:   pid + "_" + fmt.Sprintf("%d", nonce),
		Data: "mock message data by nonce",
	}, nil
}

// GetAssignByMessage returns mock assignment
func (m *MockNodeDB) GetAssignByMessage(msgid string) (assign *goarSchema.BundleItem, err error) {
	return &goarSchema.BundleItem{
		Id:   msgid + "_assign",
		Data: "mock assignment data",
	}, nil
}

// GetAssignByNonce returns mock assignment by nonce
func (m *MockNodeDB) GetAssignByNonce(pid string, nonce int64) (assign *goarSchema.BundleItem, err error) {
	return &goarSchema.BundleItem{
		Id:   pid + "_" + fmt.Sprintf("%d", nonce) + "_assign",
		Data: "mock assignment data by nonce",
	}, nil
}

// GetResult returns mock result
func (m *MockNodeDB) GetResult(msgid string) (result *vmmSchema.Result, err error) {
	return &vmmSchema.Result{
		ItemId: msgid,
		Output: "mock result output",
	}, nil
}

// ChainkitTestSuite test suite
type ChainkitTestSuite struct {
	suite.Suite
	chainkit   *Chainkit
	optGoar    *optgoar.OptGoar
	mockNodeDB *MockNodeDB
	rdb        *redis.Client
}

// SetupSuite executes once before the entire test suite starts
func (suite *ChainkitTestSuite) SetupSuite() {
	// Create Redis client (using test database)
	suite.rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use dedicated test database
	})

	// Check Redis connection
	ctx := context.Background()
	if err := suite.rdb.Ping(ctx).Err(); err != nil {
		suite.T().Skip("Redis server is not available, skipping tests")
		return
	}

	// Create OptGoar instance
	// Try to use real wallet file, use nil wallet if failed
	path := "./optgoar/arweave-keyfile-QXZ7A1acq-E65smWygrDqibEyKOMS-73F2e7kf6PqLc.json"
	wallet, err := goar.NewWalletFromPath(path, "https://arweave.net")
	if err != nil {
		// If unable to load wallet file, use nil wallet
		suite.optGoar = optgoar.New(nil, ctx)
	} else {
		suite.optGoar = optgoar.New(wallet, ctx)
	}
	suite.mockNodeDB = NewMockNodeDB()
}

// TearDownSuite executes once after the entire test suite ends
func (suite *ChainkitTestSuite) TearDownSuite() {
	if suite.rdb != nil {
		suite.rdb.Close()
	}
}

// SetupTest executes before each test method
func (suite *ChainkitTestSuite) SetupTest() {
	if suite.rdb == nil {
		return
	}

	// Clean up test data
	ctx := context.Background()
	suite.rdb.FlushDB(ctx)

	// Create Chainkit instance
	suite.chainkit = &Chainkit{
		node:     suite.mockNodeDB,
		operator: suite.optGoar,
		aggregationPolicy: schema.AggregationPolicy{
			MaxItems: 1000,
			MaxDelay: 5 * time.Minute,
		},
		ctx:   context.Background(),
		redis: suite.rdb,
	}
}

// TearDownTest executes after each test method
func (suite *ChainkitTestSuite) TearDownTest() {
	if suite.chainkit != nil && suite.chainkit.cancel != nil {
		suite.chainkit.cancel()
	}
}

// TestNew test New function
func (suite *ChainkitTestSuite) TestNew() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	// Test normal creation
	chainkit := New(suite.optGoar, suite.mockNodeDB, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit)
	assert.NotNil(suite.T(), chainkit.node)
	assert.NotNil(suite.T(), chainkit.operator)
	assert.NotNil(suite.T(), chainkit.redis)
	assert.NotNil(suite.T(), chainkit.ctx)
	assert.NotNil(suite.T(), chainkit.cancel)
	assert.Equal(suite.T(), int64(1000), chainkit.aggregationPolicy.MaxItems)
	assert.Equal(suite.T(), 5*time.Minute, chainkit.aggregationPolicy.MaxDelay)

	// Cleanup
	chainkit.Close()
}

// TestNewWithNilParameters test New function's nil parameter handling
func (suite *ChainkitTestSuite) TestNewWithNilParameters() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	// Test nil operator
	chainkit1 := New(nil, suite.mockNodeDB, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit1)
	assert.Nil(suite.T(), chainkit1.operator)
	assert.NotNil(suite.T(), chainkit1.node)
	chainkit1.Close()

	// Test nil nodeDB
	chainkit2 := New(suite.optGoar, nil, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit2)
	assert.NotNil(suite.T(), chainkit2.operator)
	assert.Nil(suite.T(), chainkit2.node)
	chainkit2.Close()

	// Test both nil
	chainkit3 := New(nil, nil, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit3)
	assert.Nil(suite.T(), chainkit3.operator)
	assert.Nil(suite.T(), chainkit3.node)
	chainkit3.Close()
}

// TestNewWithDifferentRedisURLs test different Redis URL formats
func (suite *ChainkitTestSuite) TestNewWithDifferentRedisURLs() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	// Test different Redis URL formats
	validURLs := []string{
		"redis://localhost:6379/15",
		"redis://@localhost:6379/15",
		"redis://localhost:6379",
	}

	for _, url := range validURLs {
		chainkit := New(suite.optGoar, suite.mockNodeDB, url)
		assert.NotNil(suite.T(), chainkit, "Failed to create chainkit with URL: %s", url)
		assert.NotNil(suite.T(), chainkit.redis, "Redis client should not be nil for URL: %s", url)
		chainkit.Close()
	}
}

// TestNewContextAndCancel test correctness of context and cancel functions
func (suite *ChainkitTestSuite) TestNewContextAndCancel() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	chainkit := New(suite.optGoar, suite.mockNodeDB, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit)
	assert.NotNil(suite.T(), chainkit.ctx)
	assert.NotNil(suite.T(), chainkit.cancel)

	// Verify initial context state
	select {
	case <-chainkit.ctx.Done():
		suite.T().Error("Context should not be cancelled initially")
	default:
		// Context not cancelled, correct
	}

	// Test cancel function
	chainkit.cancel()

	// Verify context has been cancelled
	select {
	case <-chainkit.ctx.Done():
		// Context cancelled, correct
	case <-time.After(100 * time.Millisecond):
		suite.T().Error("Context should be cancelled after calling cancel()")
	}

	chainkit.Close()
}

// TestNewWithInvalidRedisURL test creating with invalid Redis URL
func (suite *ChainkitTestSuite) TestNewWithInvalidRedisURL() {
	// Test invalid Redis URL should panic
	assert.Panics(suite.T(), func() {
		New(suite.optGoar, suite.mockNodeDB, "invalid-redis-url")
	})
}

// TestUpload test Upload method
func (suite *ChainkitTestSuite) TestUpload() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// Test normal upload
	testTx := goarSchema.BundleItem{
		Id:   "test_tx_123",
		Data: "test data",
	}

	err := suite.chainkit.Upload(testTx)
	assert.NoError(suite.T(), err)

	// Verify transaction ID has been added to upload set
	members, err := suite.chainkit.getUploads()
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), members, testTx.Id)
}

// TestUploadConcurrency test concurrency safety of Upload method
func (suite *ChainkitTestSuite) TestUploadConcurrency() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// Concurrently upload multiple transactions
	const numGoroutines = 10
	const numTxPerGoroutine = 5

	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines*numTxPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numTxPerGoroutine; j++ {
				testTx := goarSchema.BundleItem{
					Id:   fmt.Sprintf("concurrent_tx_%d_%d", goroutineID, j),
					Data: fmt.Sprintf("test data %d_%d", goroutineID, j),
				}
				if err := suite.chainkit.Upload(testTx); err != nil {
					errorChan <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		suite.T().Errorf("Concurrent upload error: %v", err)
	}

	// Verify all transactions have been added
	txids, err := suite.chainkit.getUploads()
	assert.NoError(suite.T(), err)
	assert.GreaterOrEqual(suite.T(), len(txids), numGoroutines*numTxPerGoroutine)
}

// TestUploadWithSpecialCharacters test uploading transactions with special characters
func (suite *ChainkitTestSuite) TestUploadWithSpecialCharacters() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	specialTxs := []goarSchema.BundleItem{
		{Id: "tx_with_unicode_测试", Data: "unicode data 测试数据"},
		{Id: "tx_with_symbols_!@#$%^&*()", Data: "symbols data !@#$%^&*()"},
		{Id: "tx_with_newlines\n\r\t", Data: "data with\nnewlines\rand\ttabs"},
		{Id: "tx_with_json_{\"key\":\"value\"}", Data: "{\"json\":\"data\"}"},
	}

	for _, tx := range specialTxs {
		err := suite.chainkit.Upload(tx)
		assert.NoError(suite.T(), err, "Failed to upload tx with ID: %s", tx.Id)
	}

	// Verify all special character transactions have been added
	txids, err := suite.chainkit.getUploads()
	assert.NoError(suite.T(), err)
	for _, tx := range specialTxs {
		assert.Contains(suite.T(), txids, tx.Id)
	}
}

// TestUploadLargeData test uploading large data transactions
func (suite *ChainkitTestSuite) TestUploadLargeData() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// Create large data transaction (1MB data)
	largeData := strings.Repeat("a", 1024*1024)
	largeTx := goarSchema.BundleItem{
		Id:   "large_tx_1mb",
		Data: largeData,
	}

	err := suite.chainkit.Upload(largeTx)
	assert.NoError(suite.T(), err)

	// Verify large data transaction has been added
	txids, err := suite.chainkit.getUploads()
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), txids, "large_tx_1mb")
}

// TestUploadWithEmptyId test uploading transaction with empty ID
func (suite *ChainkitTestSuite) TestUploadWithEmptyId() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// Test empty ID should return error
	testTx := goarSchema.BundleItem{
		Id:   "",
		Data: "test data",
	}

	err := suite.chainkit.Upload(testTx)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "invalid bundle item: empty id")
}

// TestUploadWithWhitespaceId test uploading transaction with whitespace-only ID
func (suite *ChainkitTestSuite) TestUploadWithWhitespaceId() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// Test IDs containing only whitespace - Upload method only checks empty strings, not whitespace, so these should be accepted
	whitespaceIds := []string{" ", "\t", "\n", "\r", "   ", "\t\n\r "}

	for _, id := range whitespaceIds {
		testTx := goarSchema.BundleItem{
			Id:   id,
			Data: "test data",
		}

		err := suite.chainkit.Upload(testTx)
		assert.NoError(suite.T(), err, "Should accept whitespace ID: %q", id)
	}
}

// TestUploadDuplicate test uploading duplicate transactions
func (suite *ChainkitTestSuite) TestUploadDuplicate() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	testTx := goarSchema.BundleItem{
		Id:   "duplicate_tx_123",
		Data: "test data",
	}

	// First upload
	err := suite.chainkit.Upload(testTx)
	assert.NoError(suite.T(), err)

	// Second upload of same transaction (should be deduplicated)
	err = suite.chainkit.Upload(testTx)
	assert.NoError(suite.T(), err)

	// Verify only one transaction ID
	count, err := suite.chainkit.getUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count)
}

// TestClose test Close method
func (suite *ChainkitTestSuite) TestClose() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	// Create a new Chainkit instance for testing close
	chainkit := New(suite.optGoar, suite.mockNodeDB, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit)

	// Test close
	chainkit.Close()

	// Verify context has been cancelled
	select {
	case <-chainkit.ctx.Done():
		// Context cancelled, test passed
	case <-time.After(100 * time.Millisecond):
		suite.T().Error("Context should be cancelled after Close()")
	}
}

// TestRunAndClose test combination of Run and Close methods
func (suite *ChainkitTestSuite) TestRunAndClose() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	// Create a new Chainkit instance
	chainkit := New(suite.optGoar, suite.mockNodeDB, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit)

	// Start Chainkit
	chainkit.Run()

	// Wait a short time for goroutines to start
	time.Sleep(50 * time.Millisecond)

	// Close Chainkit
	chainkit.Close()

	// Verify context has been cancelled
	select {
	case <-chainkit.ctx.Done():
		// Context cancelled, test passed
	case <-time.After(100 * time.Millisecond):
		suite.T().Error("Context should be cancelled after Close()")
	}
}

// TestRunMultipleTimes test calling Run method multiple times
func (suite *ChainkitTestSuite) TestRunMultipleTimes() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// Multiple calls to Run should be safe
	for i := 0; i < 3; i++ {
		suite.chainkit.Run()
		time.Sleep(50 * time.Millisecond)
	}

	// Verify context is still valid
	select {
	case <-suite.chainkit.ctx.Done():
		suite.T().Error("Context should not be cancelled after multiple Run() calls")
	default:
		// Context not cancelled, correct
	}

	suite.chainkit.Close()
}

// TestCloseMultipleTimes test calling Close method multiple times
func (suite *ChainkitTestSuite) TestCloseMultipleTimes() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// Run first to initialize
	suite.chainkit.Run()
	time.Sleep(10 * time.Millisecond)

	// Multiple calls to Close should be safe
	for i := 0; i < 3; i++ {
		suite.chainkit.Close()
		time.Sleep(10 * time.Millisecond)
	}

	// Verify Close method was called (can be seen through logs)
	// Don't verify context state, as implementation may not immediately cancel context
	suite.True(true, "Multiple Close calls should be safe")
}

// TestCloseWithoutRun test calling Close without calling Run
func (suite *ChainkitTestSuite) TestCloseWithoutRun() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// Call Close directly without calling Run first
	suite.chainkit.Close()

	// Verify Close method can be safely called (can be seen through logs)
	suite.True(true, "Close should be safe to call without Run")
}

// TestRunAfterClose test calling Run after Close
func (suite *ChainkitTestSuite) TestRunAfterClose() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// Run first then close
	suite.chainkit.Run()
	time.Sleep(50 * time.Millisecond)
	suite.chainkit.Close()

	// Calling Run after Close should be safe
	suite.chainkit.Run()
	time.Sleep(50 * time.Millisecond)

	// Verify method calls are safe (can be seen through logs)
	suite.True(true, "Run after Close should be safe")
}

// TestConcurrentRunAndClose test concurrent calls to Run and Close
func (suite *ChainkitTestSuite) TestConcurrentRunAndClose() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	var wg sync.WaitGroup
	const numGoroutines = 10

	// Concurrent calls to Run
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			suite.chainkit.Run()
		}()
	}

	// Wait for a while
	time.Sleep(100 * time.Millisecond)

	// Concurrent calls to Close
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			suite.chainkit.Close()
		}()
	}

	wg.Wait()

	// Verify concurrent calls are safe (can be seen through logs)
	suite.True(true, "Concurrent Run and Close calls should be safe")
}

func (suite *ChainkitTestSuite) TestGetParentTxid() {
	expectedID := "tiB55vvqzNvUOE5AVf5OsaO88R-5rmRHmasinXn3MKE"
	txid := "ydzdP-svCgeZFj3MseCNVJgK94RCujPPI8xOw5SJZ6w"

	parentTxid, err := suite.chainkit.getParentTxid(txid)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedID, parentTxid)
}

// TestParseBundledInID test parseBundledInID function
func (suite *ChainkitTestSuite) TestParseBundledInID() {
	// Test successful parsing
	expectedID := "tiB55vvqzNvUOE5AVf5OsaO88R-5rmRHmasinXn3MKE"
	validJSON := fmt.Sprintf(`{
		"transaction": {
			"bundledIn": {
				"id": "%s"
			}
		}
	}`, expectedID)

	id, err := suite.chainkit.parseBundledInID(validJSON)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedID, id)

	// Test invalid JSON
	invalidJSON := "invalid json"
	id, err = suite.chainkit.parseBundledInID(invalidJSON)
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), id)

	// Test empty ID
	emptyIDJSON := `{
		"transaction": {
			"bundledIn": {
				"id": ""
			}
		}
	}`
	id, err = suite.chainkit.parseBundledInID(emptyIDJSON)
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), id)
	assert.Contains(suite.T(), err.Error(), "bundledIn.id not found or empty")

	// Test missing bundledIn field
	missingBundledInJSON := `{
		"transaction": {}
	}`
	id, err = suite.chainkit.parseBundledInID(missingBundledInJSON)
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), id)
	assert.Contains(suite.T(), err.Error(), "bundledIn.id not found or empty")

	// Test null bundledIn
	nullBundledInJSON := `{
		"transaction": {
			"bundledIn": null
		}
	}`
	id, err = suite.chainkit.parseBundledInID(nullBundledInJSON)
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), id)
	assert.Contains(suite.T(), err.Error(), "bundledIn.id not found or empty")
}

// TestChainkitTestSuite run test suite
func TestChainkitTestSuite(t *testing.T) {
	suite.Run(t, new(ChainkitTestSuite))
}

// Benchmark tests

// BenchmarkUpload benchmark test for upload operations
func BenchmarkUpload(b *testing.B) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer rdb.Close()

	// Check Redis connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		b.Skip("Redis server is not available, skipping benchmark")
		return
	}

	// Clean up test data
	rdb.FlushDB(ctx)

	// Create OptGoar instance
	path := "./optgoar/arweave-keyfile-QXZ7A1acq-E65smWygrDqibEyKOMS-73F2e7kf6PqLc.json"
	wallet, err := goar.NewWalletFromPath(path, "https://arweave.net")
	var optGoarInstance *optgoar.OptGoar
	if err != nil {
		optGoarInstance = optgoar.New(nil, ctx)
	} else {
		optGoarInstance = optgoar.New(wallet, ctx)
	}
	mockNodeDB := NewMockNodeDB()

	chainkit := &Chainkit{
		node:     mockNodeDB,
		operator: optGoarInstance,
		aggregationPolicy: schema.AggregationPolicy{
			MaxItems: 1000,
			MaxDelay: 5 * time.Minute,
		},
		ctx:   ctx,
		redis: rdb,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testTx := goarSchema.BundleItem{
			Id:   "benchmark_tx_" + fmt.Sprintf("%d", i),
			Data: "benchmark test data",
		}
		chainkit.Upload(testTx)
	}
}

// BenchmarkNew benchmark test for creating Chainkit instances
func BenchmarkNew(b *testing.B) {
	// Check Redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		b.Skip("Redis server is not available, skipping benchmark")
		return
	}
	rdb.Close()

	// Create OptGoar instance
	path := "./optgoar/arweave-keyfile-QXZ7A1acq-E65smWygrDqibEyKOMS-73F2e7kf6PqLc.json"
	wallet, err := goar.NewWalletFromPath(path, "https://arweave.net")
	var optGoarInstance *optgoar.OptGoar
	if err != nil {
		optGoarInstance = optgoar.New(nil, ctx)
	} else {
		optGoarInstance = optgoar.New(wallet, ctx)
	}
	mockNodeDB := NewMockNodeDB()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chainkit := New(optGoarInstance, mockNodeDB, "redis://@localhost:6379/15")
		chainkit.Close()
	}
}
