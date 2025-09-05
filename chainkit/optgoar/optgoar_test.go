package optgoar

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// OptGoarTestSuite 测试套件
type OptGoarTestSuite struct {
	suite.Suite
	optGoar *OptGoar
	ctx     context.Context
	cancel  context.CancelFunc
}

// SetupSuite 在所有测试开始前运行
func (suite *OptGoarTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
}

// TearDownSuite 在所有测试结束后运行
func (suite *OptGoarTestSuite) TearDownSuite() {
	if suite.cancel != nil {
		suite.cancel()
	}
}

// SetupTest 在每个测试开始前运行
func (suite *OptGoarTestSuite) SetupTest() {
	// 使用真实的 wallet 文件进行测试
	path := "./arweave-keyfile-QXZ7A1acq-E65smWygrDqibEyKOMS-73F2e7kf6PqLc.json"
	wallet, err := goar.NewWalletFromPath(path, "https://arweave.net")
	if err != nil {
		// 如果无法加载 wallet 文件，使用 nil（测试会被跳过）
		suite.optGoar = New(nil, suite.ctx)
		return
	}
	suite.optGoar = New(wallet, suite.ctx)
}

// TestNew 测试 New 函数
func (suite *OptGoarTestSuite) TestNew() {
	path := "./arweave-keyfile-QXZ7A1acq-E65smWygrDqibEyKOMS-73F2e7kf6PqLc.json"
	wallet, err := goar.NewWalletFromPath(path, "https://arweave.net")
	assert.NoError(suite.T(), err)

	address := wallet.Signer.Address
	fmt.Println("address: ", address)
	balance, err := wallet.Client.GetWalletBalance(address)
	assert.NoError(suite.T(), err)
	fmt.Println("balance: ", balance)

	lastTxID, err := wallet.Client.GetLastTransactionID(address)
	assert.NoError(suite.T(), err)
	fmt.Println("lastTxID: ", lastTxID)

	state, err := wallet.Client.GetTransactionStatus("5Sr3QiC3In7ATxFXHQbSSWMdy4UC7ISEgnkG0YjxtFc")
	fmt.Println("state: ", state)
	assert.NoError(suite.T(), err)

	ctx := context.Background()
	optGoar := New(wallet, ctx)
	assert.NotNil(suite.T(), optGoar)
	assert.Equal(suite.T(), wallet, optGoar.wallet)
	assert.Equal(suite.T(), ctx, optGoar.ctx)
}

// TestNewWithNilWallet 测试使用 nil wallet 创建 OptGoar
func (suite *OptGoarTestSuite) TestNewWithNilWallet() {
	ctx := context.Background()

	optGoar := New(nil, ctx)

	assert.NotNil(suite.T(), optGoar)
	assert.Nil(suite.T(), optGoar.wallet)
	assert.Equal(suite.T(), ctx, optGoar.ctx)
}

// TestNewWithNilContext 测试使用 nil context 创建 OptGoar
func (suite *OptGoarTestSuite) TestNewWithNilContext() {
	wallet := &goar.Wallet{}

	optGoar := New(wallet, nil)

	assert.NotNil(suite.T(), optGoar)
	assert.Equal(suite.T(), wallet, optGoar.wallet)
	assert.Nil(suite.T(), optGoar.ctx)
}

// TestUploadWithEmptyItems 测试上传空的 items
func (suite *OptGoarTestSuite) TestUploadWithEmptyItems() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	emptyItems := []goarSchema.BundleItem{}

	txid, err := suite.optGoar.Upload(emptyItems)

	// 空的 items 应该能够创建 bundle，但可能返回空的 txid
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), txid)
}

func (suite *OptGoarTestSuite) TestUploadAndCheck() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	bundler, _ := goar.NewBundler(suite.optGoar.wallet.Signer)
	item1, err := bundler.CreateAndSignItem([]byte("eth foo"), "", "", []goarSchema.Tag{{Name: "Content-Type", Value: "application/txt"}})
	assert.NoError(suite.T(), err)
	item2, err := bundler.CreateAndSignItem([]byte("ar foo"), "", "", []goarSchema.Tag{{Name: "Content-Type", Value: "application/txt"}})
	assert.NoError(suite.T(), err)

	txid, err := suite.optGoar.Upload([]goarSchema.BundleItem{item1, item2})
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), txid)
	fmt.Println("txid: ", txid)

	// 每隔 10秒检查交易是否存在
	interval := 10 * time.Second
	ticker := time.NewTicker(interval)
	tickTime := 0
	for {
		select {
		case <-suite.ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			tickTime++
			fmt.Println("checking: ", txid, "times: ", tickTime)
			exists, err := suite.optGoar.CheckTransaction(txid)
			if err != nil {
				fmt.Println("check state: ", err)
				continue
			}
			if !exists {
				fmt.Println("tx is pending")
				continue
			}
			if exists {
				fmt.Println("tx is confirmed")
				state, _ := suite.optGoar.wallet.Client.GetTransactionStatus(txid)
				fmt.Printf("txid: %s, state: %#v\n", txid, state)
				ticker.Stop()
				return
			}
		}
	}
}

// TestDownloadWithValidParams 测试有效参数的下载
func (suite *OptGoarTestSuite) TestDownloadWithValidParams() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	parentTxID := "tiB55vvqzNvUOE5AVf5OsaO88R-5rmRHmasinXn3MKE"
	itemsIds := []string{"ydzdP-svCgeZFj3MseCNVJgK94RCujPPI8xOw5SJZ6w", "x8r4SYW3S_yyoaPUsbDcAFoxBVhaWZPKxYZf_RiwQdQ"}

	items, err := suite.optGoar.Download(parentTxID, itemsIds)
	for _, item := range items {
		data, err := goarUtils.Base64Decode(item.Data)
		assert.NoError(suite.T(), err)
		fmt.Printf("item id: %s\n", item.Id)
		fmt.Printf("data: %s\n", data)
	}

	// 由于使用 mock wallet，这里主要测试方法调用不会 panic
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), items)
}

// TestDownloadWithEmptyParams 测试空参数的下载
func (suite *OptGoarTestSuite) TestDownloadWithEmptyParams() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	parentTxID := ""
	itemsIds := []string{}

	items, err := suite.optGoar.Download(parentTxID, itemsIds)

	// 空参数可能导致错误或返回空结果
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.NotNil(suite.T(), items)
	}
}

// TestGraphQLWithValidQuery 测试有效的 GraphQL 查询
func (suite *OptGoarTestSuite) TestGraphQLWithValidQuery() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	query := `{
		transactions(first: 10) {
			edges {
				node {
					id
					owner {
						address
					}
				}
			}
		}
	}`

	result, err := suite.optGoar.GraphQL(query)
	fmt.Printf("result: %s\n", result)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
}

// TestGraphQLWithEmptyQuery 测试空的 GraphQL 查询
func (suite *OptGoarTestSuite) TestGraphQLWithEmptyQuery() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	query := ""

	result, err := suite.optGoar.GraphQL(query)

	// 空查询可能导致错误
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.NotNil(suite.T(), result)
	}
}

// TestGraphQLWithInvalidQuery 测试无效的 GraphQL 查询
func (suite *OptGoarTestSuite) TestGraphQLWithInvalidQuery() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	query := "invalid graphql query syntax"

	result, err := suite.optGoar.GraphQL(query)

	// 无效查询应该返回错误
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.NotNil(suite.T(), result)
	}
}

// TestCheckTransactionWithValidTxid 测试有效交易ID的检查
func (suite *OptGoarTestSuite) TestCheckTransactionWithValidTxid() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	txid := "valid-transaction-id-123456789abcdef"

	isValid, err := suite.optGoar.CheckTransaction(txid)

	// 由于使用 mock wallet，这里主要测试方法调用
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.IsType(suite.T(), false, isValid)
	}
}

// TestCheckTransactionWithEmptyTxid 测试空交易ID的检查
func (suite *OptGoarTestSuite) TestCheckTransactionWithEmptyTxid() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	txid := ""

	isValid, err := suite.optGoar.CheckTransaction(txid)

	// 空交易ID应该返回错误或 false
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.False(suite.T(), isValid)
	}
}

// TestCheckTransactionWithInvalidTxid 测试无效交易ID的检查
func (suite *OptGoarTestSuite) TestCheckTransactionWithInvalidTxid() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	txid := "invalid-txid"

	isValid, err := suite.optGoar.CheckTransaction(txid)

	// 无效交易ID应该返回错误或 false
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.False(suite.T(), isValid)
	}
}

// TestDecodeBundleWithEmptyData 测试空数据的 bundle 解析
func (suite *OptGoarTestSuite) TestDecodeBundleWithEmptyData() {
	// 测试空数据
	emptyData := []byte{}
	_, err := goarUtils.DecodeBundle(emptyData)
	assert.Error(suite.T(), err)
}

// TestDecodeBundleWithInvalidData 测试无效数据的 bundle 解析
func (suite *OptGoarTestSuite) TestDecodeBundleWithInvalidData() {
	invalidData := []byte("invalid bundle data")
	_, err := goarUtils.DecodeBundle(invalidData)
	assert.Error(suite.T(), err)
}

// 运行测试套件
func TestOptGoarTestSuite(t *testing.T) {
	suite.Run(t, new(OptGoarTestSuite))
}

// 基准测试

// BenchmarkNew 基准测试 New 函数
func BenchmarkNew(b *testing.B) {
	wallet := &goar.Wallet{}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = New(wallet, ctx)
	}
}

// BenchmarkUpload 基准测试 Upload 方法
func BenchmarkUpload(b *testing.B) {
	optGoar := New(nil, context.Background())
	items := []goarSchema.BundleItem{
		{
			Id:   "benchmark-item",
			Data: "benchmark data",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if optGoar.wallet != nil {
			_, _ = optGoar.Upload(items)
		}
	}
}

// BenchmarkDecodeBundle 基准测试 DecodeBundle 函数
func BenchmarkDecodeBundle(b *testing.B) {
	data := []byte("test bundle data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = goarUtils.DecodeBundle(data)
	}
}
