package schema

import (
	goarSchema "github.com/permadao/goar/schema"
)

// MockOperator 是 IOperator 接口的 mock 实现，所有方法都返回成功状态
type MockOperator struct{}

// NewMockOperator 创建一个新的 MockOperator 实例
func NewMockOperator() *MockOperator {
	return &MockOperator{}
}

// Upload 模拟上传交易，返回一个假的交易ID
func (m *MockOperator) Upload(items []goarSchema.BundleItem) (txid string, err error) {
	// 返回一个模拟的交易ID
	return "mock-txid-123456789abcdef", nil
}

// Download 模拟下载交易，返回一个空的 BundleItem
func (m *MockOperator) Download(parentTxID string, itemsIds []string) (items []*goarSchema.BundleItem, err error) {
	// 返回一个空的 BundleItem，表示下载成功
	return []*goarSchema.BundleItem{}, nil
}

// GraphQL 模拟执行 GraphQL 查询，返回空的 JSON 响应
func (m *MockOperator) GraphQL(query string) ([]byte, error) {
	// 返回一个简单的成功响应
	return []byte(`{"data":{}}`), nil
}

// CheckTransaction 模拟检查交易状态，总是返回成功
func (m *MockOperator) CheckTransaction(txid string) (bool, error) {
	// 总是返回 true，表示交易上传成功
	return true, nil
}
