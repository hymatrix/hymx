package schema

import (
	goarSchema "github.com/permadao/goar/schema"
)

type IOperator interface {
	// Upload a transaction in BundleItem form; a BundleItem may nest multiple BundleItems.
	// The outermost BundleItem is submitted to the network as the atomic Transaction.
	// The outermost BundleItem's txID is used as parentTxID when downloading.
	Upload(items []goarSchema.BundleItem) (txid string, err error)

	// Download a transaction
	// return BundleItem format
	Download(itemID string) (*goarSchema.BundleItem, error)

	// Download multiple transactions
	Downloads(itemIDs []string) ([]*goarSchema.BundleItem, error)

	// Execute a GraphQL query.
	GraphQL(query string) ([]byte, error)

	// Check if a transaction upload success
	CheckTransaction(txid string) (bool, error)
}
