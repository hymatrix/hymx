package chainkit

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// DBTestSuite 数据库测试套件
type DBTestSuite struct {
	suite.Suite
	chainkit *Chainkit
	rdb      *redis.Client
}

// SetupSuite 测试套件初始化
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

	// 测试 Redis 连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = suite.rdb.Ping(ctx).Result()
	if err != nil {
		suite.T().Skip("Redis server not available, skipping tests")
	}
}

// TearDownSuite 测试套件清理
func (suite *DBTestSuite) TearDownSuite() {
	if suite.rdb != nil {
		suite.rdb.Close()
	}
}

// SetupTest 每个测试前的准备工作
func (suite *DBTestSuite) SetupTest() {
	// 清理测试数据
	suite.rdb.FlushDB(context.Background())
}

// TestGetUploads 测试获取待上传集合
func (suite *DBTestSuite) TestGetUploads() {
	// 测试空集合
	members, err := suite.chainkit.GetUploads()
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), members)

	// 添加测试数据
	testTxIDs := []string{"tx1", "tx2", "tx3"}
	for _, txid := range testTxIDs {
		err := suite.chainkit.AddToUploads(txid)
		assert.NoError(suite.T(), err)
	}

	// 测试获取所有成员
	members, err = suite.chainkit.GetUploads()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), members, 3)
	for _, txid := range testTxIDs {
		assert.Contains(suite.T(), members, txid)
	}
}

// TestGetUploadsCount 测试获取待上传集合数量
func (suite *DBTestSuite) TestGetUploadsCount() {
	// 测试空集合
	count, err := suite.chainkit.GetUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), count)

	// 添加测试数据
	testTxIDs := []string{"tx1", "tx2", "tx3"}
	for _, txid := range testTxIDs {
		err := suite.chainkit.AddToUploads(txid)
		assert.NoError(suite.T(), err)
	}

	// 测试计数
	count, err = suite.chainkit.GetUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(3), count)
}

// TestAddToUploads 测试添加到待上传集合
func (suite *DBTestSuite) TestAddToUploads() {
	testTxID := "test_tx_123"

	// 测试添加
	err := suite.chainkit.AddToUploads(testTxID)
	assert.NoError(suite.T(), err)

	// 验证添加成功
	members, err := suite.chainkit.GetUploads()
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), members, testTxID)

	// 测试重复添加（Set 特性，不会重复）
	err = suite.chainkit.AddToUploads(testTxID)
	assert.NoError(suite.T(), err)

	count, err := suite.chainkit.GetUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count)
}

// TestMoveToPending 测试移动到待确认集合
func (suite *DBTestSuite) TestMoveToPending() {
	// 准备测试数据
	testTxIDs := []string{"tx1", "tx2", "tx3"}
	parentTxID := "parent_tx_123"

	// 先添加到待上传集合
	for _, txid := range testTxIDs {
		err := suite.chainkit.AddToUploads(txid)
		assert.NoError(suite.T(), err)
	}

	// 验证初始状态
	count, err := suite.chainkit.GetUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(3), count)

	// 执行移动操作
	err = suite.chainkit.MoveToPending(parentTxID, testTxIDs)
	assert.NoError(suite.T(), err)

	// 验证从待上传集合中移除
	count, err = suite.chainkit.GetUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), count)

	// 验证添加到待确认集合
	childTxIDs, err := suite.chainkit.GetPendingSub(parentTxID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs, 3)
	for _, txid := range testTxIDs {
		assert.Contains(suite.T(), childTxIDs, txid)
	}
}

// TestGetPendings 测试获取所有待确认的父交易ID
func (suite *DBTestSuite) TestGetPendings() {
	// 测试空状态
	parentTxIDs, err := suite.chainkit.GetPendings()
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), parentTxIDs)

	// 添加测试数据
	testParents := []string{"parent1", "parent2", "parent3"}
	for _, parent := range testParents {
		testTxIDs := []string{"child1_" + parent, "child2_" + parent}
		err := suite.chainkit.MoveToPending(parent, testTxIDs)
		assert.NoError(suite.T(), err)
	}

	// 测试获取所有父交易ID
	parentTxIDs, err = suite.chainkit.GetPendings()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), parentTxIDs, 3)
	for _, parent := range testParents {
		assert.Contains(suite.T(), parentTxIDs, parent)
	}
}

// TestGetPendingSub 测试获取子交易ID
func (suite *DBTestSuite) TestGetPendingSub() {
	parentTxID := "parent_tx_456"
	testTxIDs := []string{"child1", "child2", "child3"}

	// 测试不存在的父交易
	childTxIDs, err := suite.chainkit.GetPendingSub("nonexistent")
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), childTxIDs)

	// 添加测试数据
	err = suite.chainkit.MoveToPending(parentTxID, testTxIDs)
	assert.NoError(suite.T(), err)

	// 测试获取子交易ID
	childTxIDs, err = suite.chainkit.GetPendingSub(parentTxID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs, 3)
	for _, txid := range testTxIDs {
		assert.Contains(suite.T(), childTxIDs, txid)
	}
}

// TestRemovePending 测试移除待确认交易
func (suite *DBTestSuite) TestRemovePending() {
	parentTxID := "parent_tx_789"
	testTxIDs := []string{"child1", "child2", "child3"}

	// 添加测试数据
	err := suite.chainkit.MoveToPending(parentTxID, testTxIDs)
	assert.NoError(suite.T(), err)

	// 验证数据存在
	childTxIDs, err := suite.chainkit.GetPendingSub(parentTxID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs, 3)

	// 执行删除操作
	err = suite.chainkit.RemovePending(parentTxID)
	assert.NoError(suite.T(), err)

	// 验证删除成功
	childTxIDs, err = suite.chainkit.GetPendingSub(parentTxID)
	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), childTxIDs)

	// 验证从父交易列表中移除
	parentTxIDs, err := suite.chainkit.GetPendings()
	assert.NoError(suite.T(), err)
	assert.NotContains(suite.T(), parentTxIDs, parentTxID)
}

// TestComplexWorkflow 测试复杂工作流程
func (suite *DBTestSuite) TestComplexWorkflow() {
	// 模拟完整的工作流程
	testTxIDs := []string{"tx1", "tx2", "tx3", "tx4", "tx5"}
	parentTxID1 := "parent1"
	parentTxID2 := "parent2"

	// 1. 添加交易到待上传集合
	for _, txid := range testTxIDs {
		err := suite.chainkit.AddToUploads(txid)
		assert.NoError(suite.T(), err)
	}

	// 2. 验证待上传集合状态
	count, err := suite.chainkit.GetUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(5), count)

	// 3. 分批移动到待确认集合
	err = suite.chainkit.MoveToPending(parentTxID1, testTxIDs[:3])
	assert.NoError(suite.T(), err)

	err = suite.chainkit.MoveToPending(parentTxID2, testTxIDs[3:])
	assert.NoError(suite.T(), err)

	// 4. 验证待上传集合已清空
	count, err = suite.chainkit.GetUploadsCount()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), count)

	// 5. 验证待确认集合状态
	parentTxIDs, err := suite.chainkit.GetPendings()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), parentTxIDs, 2)
	assert.Contains(suite.T(), parentTxIDs, parentTxID1)
	assert.Contains(suite.T(), parentTxIDs, parentTxID2)

	// 6. 验证子交易分组
	childTxIDs1, err := suite.chainkit.GetPendingSub(parentTxID1)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs1, 3)

	childTxIDs2, err := suite.chainkit.GetPendingSub(parentTxID2)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), childTxIDs2, 2)

	// 7. 移除一个父交易
	err = suite.chainkit.RemovePending(parentTxID1)
	assert.NoError(suite.T(), err)

	// 8. 验证最终状态
	parentTxIDs, err = suite.chainkit.GetPendings()
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), parentTxIDs, 1)
	assert.Contains(suite.T(), parentTxIDs, parentTxID2)
}

// TestEdgeCases 测试边界情况
func (suite *DBTestSuite) TestEdgeCases() {
	// 测试空字符串
	err := suite.chainkit.AddToUploads("")
	assert.NoError(suite.T(), err)

	// 测试特殊字符
	specialTxID := "tx:with:colons"
	err = suite.chainkit.AddToUploads(specialTxID)
	assert.NoError(suite.T(), err)

	members, err := suite.chainkit.GetUploads()
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), members, specialTxID)

	// 测试移动空数组
	err = suite.chainkit.MoveToPending("empty_parent", []string{})
	assert.NoError(suite.T(), err)

	// 测试删除不存在的父交易
	err = suite.chainkit.RemovePending("nonexistent_parent")
	assert.NoError(suite.T(), err)
}

// TestDBTestSuite 运行测试套件
func TestDBTestSuite(t *testing.T) {
	suite.Run(t, new(DBTestSuite))
}

// 基准测试

// BenchmarkAddToUploads 添加操作的基准测试
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

	// 清理测试数据
	rdb.FlushDB(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txid := "benchmark_tx_" + string(rune(i))
		chainkit.AddToUploads(txid)
	}
}

// BenchmarkGetUploads 获取操作的基准测试
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

	// 准备测试数据
	rdb.FlushDB(context.Background())
	for i := 0; i < 1000; i++ {
		txid := "benchmark_tx_" + string(rune(i))
		chainkit.AddToUploads(txid)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chainkit.GetUploads()
	}
}
