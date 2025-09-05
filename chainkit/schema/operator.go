package schema

import (
	goarSchema "github.com/permadao/goar/schema"
)

type IOperator interface {
	// Upload a transaction in BundleItem form; a BundleItem may nest multiple BundleItems.
	// The outermost BundleItem is submitted to the network as the atomic Transaction.
	// The outermost BundleItem's txID is used as parentTxID when downloading.
	Upload(items []goarSchema.BundleItem) (txid string, err error)

	// Download a transaction in BundleItem form.
	// parentTxID is the txID of the outermost BundleItem.
	// childTxID is the txID of the innermost BundleItem that points to the original transaction.
	Download(parentTxID string, itemsIds []string) (items []*goarSchema.BundleItem, err error)

	// Execute a GraphQL query.
	GraphQL(query string) ([]byte, error)

	// Check if a transaction upload success
	CheckTransaction(txid string) (bool, error)
}
