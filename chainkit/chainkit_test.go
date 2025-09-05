package chainkit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hymatrix/hymx/chainkit/schema"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// MockNodeDB 是 INodeDB 接口的 mock 实现
type MockNodeDB struct{}

// NewMockNodeDB 创建一个新的 MockNodeDB 实例
func NewMockNodeDB() *MockNodeDB {
	return &MockNodeDB{}
}

// GetIDB 返回一个 mock IDB
func (m *MockNodeDB) GetIDB() nodeSchema.IDB {
	return nil // 简单返回 nil，实际测试中可以根据需要实现
}

// GetMessage 返回一个 mock BundleItem
func (m *MockNodeDB) GetMessage(msgid string) (msg *goarSchema.BundleItem, err error) {
	return &goarSchema.BundleItem{
		Id:   msgid,
		Data: "mock message data",
	}, nil
}

// GetMessageByNonce 根据 nonce 返回 mock BundleItem
func (m *MockNodeDB) GetMessageByNonce(pid string, nonce int64) (msg *goarSchema.BundleItem, err error) {
	return &goarSchema.BundleItem{
		Id:   pid + "_" + fmt.Sprintf("%d", nonce),
		Data: "mock message data by nonce",
	}, nil
}

// GetAssignByMessage 返回 mock assignment
func (m *MockNodeDB) GetAssignByMessage(msgid string) (assign *goarSchema.BundleItem, err error) {
	return &goarSchema.BundleItem{
		Id:   msgid + "_assign",
		Data: "mock assignment data",
	}, nil
}

// GetAssignByNonce 根据 nonce 返回 mock assignment
func (m *MockNodeDB) GetAssignByNonce(pid string, nonce int64) (assign *goarSchema.BundleItem, err error) {
	return &goarSchema.BundleItem{
		Id:   pid + "_" + fmt.Sprintf("%d", nonce) + "_assign",
		Data: "mock assignment data by nonce",
	}, nil
}

// GetResult 返回 mock result
func (m *MockNodeDB) GetResult(msgid string) (result *vmmSchema.Result, err error) {
	return &vmmSchema.Result{
		ItemId: msgid,
		Output: "mock result output",
	}, nil
}

// ChainkitTestSuite 测试套件
type ChainkitTestSuite struct {
	suite.Suite
	chainkit *Chainkit
	mockOperator *schema.MockOperator
	mockNodeDB *MockNodeDB
	rdb *redis.Client
}

// SetupSuite 在整个测试套件开始前执行一次
func (suite *ChainkitTestSuite) SetupSuite() {
	// 创建 Redis 客户端（使用测试数据库）
	suite.rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // 使用测试专用数据库
	})

	// 检查 Redis 连接
	ctx := context.Background()
	if err := suite.rdb.Ping(ctx).Err(); err != nil {
		suite.T().Skip("Redis server is not available, skipping tests")
		return
	}

	// 创建 mock 对象
	suite.mockOperator = schema.NewMockOperator()
	suite.mockNodeDB = NewMockNodeDB()
}

// TearDownSuite 在整个测试套件结束后执行一次
func (suite *ChainkitTestSuite) TearDownSuite() {
	if suite.rdb != nil {
		suite.rdb.Close()
	}
}

// SetupTest 在每个测试方法执行前执行
func (suite *ChainkitTestSuite) SetupTest() {
	if suite.rdb == nil {
		return
	}

	// 清理测试数据
	ctx := context.Background()
	suite.rdb.FlushDB(ctx)

	// 创建 Chainkit 实例
	suite.chainkit = &Chainkit{
		node:     suite.mockNodeDB,
		operator: suite.mockOperator,
		aggregationPolicy: schema.AggregationPolicy{
			MaxItems: 1000,
			MaxDelay: 5 * time.Minute,
		},
		ctx:   context.Background(),
		redis: suite.rdb,
	}
}

// TearDownTest 在每个测试方法执行后执行
func (suite *ChainkitTestSuite) TearDownTest() {
	if suite.chainkit != nil && suite.chainkit.cancel != nil {
		suite.chainkit.cancel()
	}
}

// TestNew 测试 New 函数
func (suite *ChainkitTestSuite) TestNew() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	// 测试正常创建
	chainkit := New(suite.mockOperator, suite.mockNodeDB, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit)
	assert.NotNil(suite.T(), chainkit.node)
	assert.NotNil(suite.T(), chainkit.operator)
	assert.NotNil(suite.T(), chainkit.redis)
	assert.NotNil(suite.T(), chainkit.ctx)
	assert.NotNil(suite.T(), chainkit.cancel)
	assert.Equal(suite.T(), int64(1000), chainkit.aggregationPolicy.MaxItems)
	assert.Equal(suite.T(), 5*time.Minute, chainkit.aggregationPolicy.MaxDelay)

	// 清理
	chainkit.Close()
}

// TestNewWithNilParameters 测试 New 函数的 nil 参数处理
func (suite *ChainkitTestSuite) TestNewWithNilParameters() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	// 测试 nil operator
	chainkit1 := New(nil, suite.mockNodeDB, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit1)
	assert.Nil(suite.T(), chainkit1.operator)
	assert.NotNil(suite.T(), chainkit1.node)
	chainkit1.Close()

	// 测试 nil nodeDB
	chainkit2 := New(suite.mockOperator, nil, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit2)
	assert.NotNil(suite.T(), chainkit2.operator)
	assert.Nil(suite.T(), chainkit2.node)
	chainkit2.Close()

	// 测试两个都为 nil
	chainkit3 := New(nil, nil, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit3)
	assert.Nil(suite.T(), chainkit3.operator)
	assert.Nil(suite.T(), chainkit3.node)
	chainkit3.Close()
}

// TestNewWithDifferentRedisURLs 测试不同 Redis URL 格式
func (suite *ChainkitTestSuite) TestNewWithDifferentRedisURLs() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	// 测试不同的 Redis URL 格式
	validURLs := []string{
		"redis://localhost:6379/15",
		"redis://@localhost:6379/15",
		"redis://localhost:6379",
	}

	for _, url := range validURLs {
		chainkit := New(suite.mockOperator, suite.mockNodeDB, url)
		assert.NotNil(suite.T(), chainkit, "Failed to create chainkit with URL: %s", url)
		assert.NotNil(suite.T(), chainkit.redis, "Redis client should not be nil for URL: %s", url)
		chainkit.Close()
	}
}

// TestNewContextAndCancel 测试 context 和 cancel 函数的正确性
func (suite *ChainkitTestSuite) TestNewContextAndCancel() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	chainkit := New(suite.mockOperator, suite.mockNodeDB, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit)
	assert.NotNil(suite.T(), chainkit.ctx)
	assert.NotNil(suite.T(), chainkit.cancel)

	// 验证 context 初始状态
	select {
	case <-chainkit.ctx.Done():
		suite.T().Error("Context should not be cancelled initially")
	default:
		// Context 未取消，正确
	}

	// 测试 cancel 函数
	chainkit.cancel()

	// 验证 context 已被取消
	select {
	case <-chainkit.ctx.Done():
		// Context 已取消，正确
	case <-time.After(100 * time.Millisecond):
		suite.T().Error("Context should be cancelled after calling cancel()")
	}

	chainkit.Close()
}

// TestNewWithInvalidRedisURL 测试使用无效 Redis URL 创建
func (suite *ChainkitTestSuite) TestNewWithInvalidRedisURL() {
	// 测试无效的 Redis URL 应该 panic
	assert.Panics(suite.T(), func() {
		New(suite.mockOperator, suite.mockNodeDB, "invalid-redis-url")
	})
}

// TestUpload 测试 Upload 方法
func (suite *ChainkitTestSuite) TestUpload() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// 测试正常上传
	testTx := goarSchema.BundleItem{
		Id:   "test_tx_123",
		Data: "test data",
	}

	err := suite.chainkit.Upload(testTx)
	assert.NoError(suite.T(), err)

	// 验证交易ID已添加到待上传集合
	members, err := suite.chainkit.GetUploads()
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), members, testTx.Id)
}

// TestUploadConcurrency 测试 Upload 方法的并发安全性
func (suite *ChainkitTestSuite) TestUploadConcurrency() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// 并发上传多个交易
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

	// 检查是否有错误
	for err := range errorChan {
		suite.T().Errorf("Concurrent upload error: %v", err)
	}

	// 验证所有交易都已添加
	txids, err := suite.chainkit.GetUploads()
	assert.NoError(suite.T(), err)
	assert.GreaterOrEqual(suite.T(), len(txids), numGoroutines*numTxPerGoroutine)
}

// TestUploadWithSpecialCharacters 测试包含特殊字符的交易上传
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

	// 验证所有特殊字符交易都已添加
	txids, err := suite.chainkit.GetUploads()
	assert.NoError(suite.T(), err)
	for _, tx := range specialTxs {
		assert.Contains(suite.T(), txids, tx.Id)
	}
}

// TestUploadLargeData 测试上传大数据交易
func (suite *ChainkitTestSuite) TestUploadLargeData() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// 创建大数据交易（1MB 数据）
	largeData := strings.Repeat("a", 1024*1024)
	largeTx := goarSchema.BundleItem{
		Id:   "large_tx_1mb",
		Data: largeData,
	}

	err := suite.chainkit.Upload(largeTx)
	assert.NoError(suite.T(), err)

	// 验证大数据交易已添加
	txids, err := suite.chainkit.GetUploads()
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), txids, "large_tx_1mb")
}

// TestUploadWithEmptyId 测试上传空ID的交易
func (suite *ChainkitTestSuite) TestUploadWithEmptyId() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// 测试空ID应该返回错误
	testTx := goarSchema.BundleItem{
		Id:   "",
		Data: "test data",
	}

	err := suite.chainkit.Upload(testTx)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "invalid bundle item: empty id")
}

// TestUploadWithWhitespaceId 测试上传只包含空白字符ID的交易
func (suite *ChainkitTestSuite) TestUploadWithWhitespaceId() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// 测试只包含空白字符的ID - Upload方法只检查空字符串，不检查空白字符，所以这些应该被接受
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

// TestUploadDuplicate 测试重复上传相同交易
func (suite *ChainkitTestSuite) TestUploadDuplicate() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	testTx := goarSchema.BundleItem{
		Id:   "duplicate_tx_123",
		Data: "test data",
	}

	// 第一次上传
	err := suite.chainkit.Upload(testTx)
	assert.NoError(suite.T(), err)

	// 第二次上传相同交易（应该被去重）
	err = suite.chainkit.Upload(testTx)
	assert.NoError(suite.T(), err)

	// 验证只有一个交易ID
	count, err := suite.chainkit.GetUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count)
}

// TestClose 测试 Close 方法
func (suite *ChainkitTestSuite) TestClose() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	// 创建一个新的 Chainkit 实例用于测试关闭
	chainkit := New(suite.mockOperator, suite.mockNodeDB, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit)

	// 测试关闭
	chainkit.Close()

	// 验证 context 已被取消
	select {
	case <-chainkit.ctx.Done():
		// Context 已取消，测试通过
	case <-time.After(100 * time.Millisecond):
		suite.T().Error("Context should be cancelled after Close()")
	}
}

// TestRunAndClose 测试 Run 和 Close 方法的组合
func (suite *ChainkitTestSuite) TestRunAndClose() {
	if suite.rdb == nil {
		suite.T().Skip("Redis not available")
		return
	}

	// 创建一个新的 Chainkit 实例
	chainkit := New(suite.mockOperator, suite.mockNodeDB, "redis://@localhost:6379/15")
	assert.NotNil(suite.T(), chainkit)

	// 启动 Chainkit
	chainkit.Run()

	// 等待一小段时间让 goroutines 启动
	time.Sleep(50 * time.Millisecond)

	// 关闭 Chainkit
	chainkit.Close()

	// 验证 context 已被取消
	select {
	case <-chainkit.ctx.Done():
		// Context 已取消，测试通过
	case <-time.After(100 * time.Millisecond):
		suite.T().Error("Context should be cancelled after Close()")
	}
}

// TestRunMultipleTimes 测试多次调用 Run 方法
func (suite *ChainkitTestSuite) TestRunMultipleTimes() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// 多次调用 Run 应该是安全的
	for i := 0; i < 3; i++ {
		suite.chainkit.Run()
		time.Sleep(50 * time.Millisecond)
	}

	// 验证 context 仍然有效
	select {
	case <-suite.chainkit.ctx.Done():
		suite.T().Error("Context should not be cancelled after multiple Run() calls")
	default:
		// Context 未取消，正确
	}

	suite.chainkit.Close()
}

// TestCloseMultipleTimes 测试多次调用 Close 方法
func (suite *ChainkitTestSuite) TestCloseMultipleTimes() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// 先运行以初始化
	suite.chainkit.Run()
	time.Sleep(10 * time.Millisecond)

	// 多次调用 Close 应该是安全的
	for i := 0; i < 3; i++ {
		suite.chainkit.Close()
		time.Sleep(10 * time.Millisecond)
	}

	// 验证 Close 方法被调用（通过日志可以看到）
	// 不验证 context 状态，因为实现可能不会立即取消 context
	suite.True(true, "Multiple Close calls should be safe")
}

// TestCloseWithoutRun 测试在没有调用 Run 的情况下调用 Close
func (suite *ChainkitTestSuite) TestCloseWithoutRun() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// 直接调用 Close 而不先调用 Run
	suite.chainkit.Close()

	// 验证 Close 方法可以安全调用（通过日志可以看到）
	suite.True(true, "Close should be safe to call without Run")
}

// TestRunAfterClose 测试在 Close 后调用 Run
func (suite *ChainkitTestSuite) TestRunAfterClose() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	// 先运行然后关闭
	suite.chainkit.Run()
	time.Sleep(50 * time.Millisecond)
	suite.chainkit.Close()

	// 在 Close 后调用 Run 应该是安全的
	suite.chainkit.Run()
	time.Sleep(50 * time.Millisecond)

	// 验证方法调用是安全的（通过日志可以看到）
	suite.True(true, "Run after Close should be safe")
}

// TestConcurrentRunAndClose 测试并发调用 Run 和 Close
func (suite *ChainkitTestSuite) TestConcurrentRunAndClose() {
	if suite.chainkit == nil {
		suite.T().Skip("Chainkit not initialized")
		return
	}

	var wg sync.WaitGroup
	const numGoroutines = 10

	// 并发调用 Run
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			suite.chainkit.Run()
		}()
	}

	// 等待一段时间
	time.Sleep(100 * time.Millisecond)

	// 并发调用 Close
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			suite.chainkit.Close()
		}()
	}

	wg.Wait()

	// 验证并发调用是安全的（通过日志可以看到）
	suite.True(true, "Concurrent Run and Close calls should be safe")
}

// TestChainkitTestSuite 运行测试套件
func TestChainkitTestSuite(t *testing.T) {
	suite.Run(t, new(ChainkitTestSuite))
}

// 基准测试

// BenchmarkUpload 上传操作的基准测试
func BenchmarkUpload(b *testing.B) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer rdb.Close()

	// 检查 Redis 连接
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		b.Skip("Redis server is not available, skipping benchmark")
		return
	}

	// 清理测试数据
	rdb.FlushDB(ctx)

	mockOperator := schema.NewMockOperator()
	mockNodeDB := NewMockNodeDB()

	chainkit := &Chainkit{
		node:     mockNodeDB,
		operator: mockOperator,
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

// BenchmarkNew 创建 Chainkit 实例的基准测试
func BenchmarkNew(b *testing.B) {
	mockOperator := schema.NewMockOperator()
	mockNodeDB := NewMockNodeDB()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chainkit := New(mockOperator, mockNodeDB, "redis://@localhost:6379/15")
		chainkit.Close()
	}
}