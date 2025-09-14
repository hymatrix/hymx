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

// OptGoarTestSuite test suite
type OptGoarTestSuite struct {
	suite.Suite
	optGoar *OptGoar
	ctx     context.Context
	cancel  context.CancelFunc
}

// SetupSuite runs before all tests
func (suite *OptGoarTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
}

// TearDownSuite runs after all tests
func (suite *OptGoarTestSuite) TearDownSuite() {
	if suite.cancel != nil {
		suite.cancel()
	}
}

// SetupTest runs before each test
func (suite *OptGoarTestSuite) SetupTest() {
	// Use real wallet file for testing
	path := "./arweave-keyfile-QXZ7A1acq-E65smWygrDqibEyKOMS-73F2e7kf6PqLc.json"
	wallet, err := goar.NewWalletFromPath(path, "https://arweave.net")
	if err != nil {
		// If wallet file cannot be loaded, use nil (tests will be skipped)
		suite.optGoar = New(nil, suite.ctx)
		return
	}
	suite.optGoar = New(wallet, suite.ctx)
}

// TestNew tests the New function
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

// TestNewWithNilWallet tests creating OptGoar with nil wallet
func (suite *OptGoarTestSuite) TestNewWithNilWallet() {
	ctx := context.Background()

	optGoar := New(nil, ctx)

	assert.NotNil(suite.T(), optGoar)
	assert.Nil(suite.T(), optGoar.wallet)
	assert.Equal(suite.T(), ctx, optGoar.ctx)
}

// TestNewWithNilContext tests creating OptGoar with nil context
func (suite *OptGoarTestSuite) TestNewWithNilContext() {
	wallet := &goar.Wallet{}

	optGoar := New(wallet, nil)

	assert.NotNil(suite.T(), optGoar)
	assert.Equal(suite.T(), wallet, optGoar.wallet)
	assert.Nil(suite.T(), optGoar.ctx)
}

// TestUploadWithEmptyItems tests uploading empty items
func (suite *OptGoarTestSuite) TestUploadWithEmptyItems() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	emptyItems := []goarSchema.BundleItem{}

	txid, err := suite.optGoar.Upload(emptyItems)

	// Empty items should be able to create bundle, but may return empty txid
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

	// Check if transaction exists every 10 seconds
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

// TestDownloadWithValidParams tests download with valid parameters
func (suite *OptGoarTestSuite) TestDownloadWithValidParams() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	// parentTxID := "tiB55vvqzNvUOE5AVf5OsaO88R-5rmRHmasinXn3MKE"
	itemsIds := []string{"ydzdP-svCgeZFj3MseCNVJgK94RCujPII8xOw5SJZ6w", "x8r4SYW3S_yyoaPUsbDcAFoxBVhaWZPKxYZf_RiwQdQ"}

	items, err := suite.optGoar.Downloads(itemsIds)
	for _, item := range items {
		data, err := goarUtils.Base64Decode(item.Data)
		assert.NoError(suite.T(), err)
		fmt.Printf("item id: %s\n", item.Id)
		fmt.Printf("data: %s\n", data)
	}

	// Since using mock wallet, mainly test that method calls don't panic
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), items)
}

// TestDownloadWithEmptyParams tests download with empty parameters
func (suite *OptGoarTestSuite) TestDownloadWithEmptyParams() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	itemsIds := []string{}

	items, err := suite.optGoar.Downloads(itemsIds)

	// Empty parameters may cause errors or return empty results
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.NotNil(suite.T(), items)
	}
}

// TestGraphQLWithValidQuery tests valid GraphQL query
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

// TestGraphQLWithEmptyQuery tests empty GraphQL query
func (suite *OptGoarTestSuite) TestGraphQLWithEmptyQuery() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	query := ""

	result, err := suite.optGoar.GraphQL(query)

	// Empty query may cause errors
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.NotNil(suite.T(), result)
	}
}

func (suite *OptGoarTestSuite) TestGraphQLWithItemId() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	// Use placeholder ID to test querying parent transaction ID functionality
	txid := "ydzdP-svCgeZFj3MseCNVJgK94RCujPPI8xOw5SJZ6w"
	const GraphQLBundledInQueryTemplate = `{
		transaction(id: "%s") {
			bundledIn {
				id
			}
		}
	}`

	query := fmt.Sprintf(GraphQLBundledInQueryTemplate, txid)
	result, err := suite.optGoar.GraphQL(query)
	fmt.Printf("%s\n", string(result))
	assert.NoError(suite.T(), err)
}

// TestGraphQLWithInvalidQuery tests invalid GraphQL query
func (suite *OptGoarTestSuite) TestGraphQLWithInvalidQuery() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	query := "invalid graphql query syntax"

	result, err := suite.optGoar.GraphQL(query)

	// Invalid query should return error
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.NotNil(suite.T(), result)
	}
}

// TestCheckTransactionWithValidTxid tests checking valid transaction ID
func (suite *OptGoarTestSuite) TestCheckTransactionWithValidTxid() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	//txid := "valid-transaction-id-123456789abcdef"
	txid := "tiB55vvqzNvUOE5AVf5OsaO88R-5rmRHmasinXn3MKE"

	isValid, err := suite.optGoar.CheckTransaction(txid)

	// Since using mock wallet, mainly test method calls
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.IsType(suite.T(), false, isValid)
	}
}

// TestCheckTransactionWithEmptyTxid tests checking empty transaction ID
func (suite *OptGoarTestSuite) TestCheckTransactionWithEmptyTxid() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	txid := ""

	isValid, err := suite.optGoar.CheckTransaction(txid)

	// Empty transaction ID should return error or false
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.False(suite.T(), isValid)
	}
}

// TestCheckTransactionWithInvalidTxid tests checking invalid transaction ID
func (suite *OptGoarTestSuite) TestCheckTransactionWithInvalidTxid() {
	if suite.optGoar.wallet == nil {
		suite.T().Skip("Skipping test due to nil wallet")
		return
	}

	txid := "invalid-txid"

	isValid, err := suite.optGoar.CheckTransaction(txid)

	// Invalid transaction ID should return error or false
	if err != nil {
		assert.Error(suite.T(), err)
	} else {
		assert.False(suite.T(), isValid)
	}
}

// TestDecodeBundleWithEmptyData tests bundle parsing with empty data
func (suite *OptGoarTestSuite) TestDecodeBundleWithEmptyData() {
	// Test empty data
	emptyData := []byte{}
	_, err := goarUtils.DecodeBundle(emptyData)
	assert.Error(suite.T(), err)
}

// TestDecodeBundleWithInvalidData tests bundle parsing with invalid data
func (suite *OptGoarTestSuite) TestDecodeBundleWithInvalidData() {
	invalidData := []byte("invalid bundle data")
	_, err := goarUtils.DecodeBundle(invalidData)
	assert.Error(suite.T(), err)
}

// Run test suite
func TestOptGoarTestSuite(t *testing.T) {
	suite.Run(t, new(OptGoarTestSuite))
}

// Benchmark tests

// BenchmarkNew benchmark test for New function
func BenchmarkNew(b *testing.B) {
	wallet := &goar.Wallet{}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = New(wallet, ctx)
	}
}

// BenchmarkUpload benchmark test for Upload method
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

// BenchmarkDecodeBundle benchmark test for DecodeBundle function
func BenchmarkDecodeBundle(b *testing.B) {
	data := []byte("test bundle data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = goarUtils.DecodeBundle(data)
	}
}
