package optgoar

import (
	"context"
	"fmt"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

type OptGoar struct {
	ctx    context.Context
	wallet *goar.Wallet
}

func New(wallet *goar.Wallet, ctx context.Context) *OptGoar {
	return &OptGoar{
		wallet: wallet,
		ctx:    ctx,
	}
}

func (o *OptGoar) Upload(items []goarSchema.BundleItem) (txid string, err error) {
	// create bundle
	bundle, err := goarUtils.NewBundle(items...)
	if err != nil {
		return "", err
	}

	// send to arweave
	tx, err := o.wallet.SendBundleTx(o.ctx, 1, bundle.Binary, []goarSchema.Tag{
		{Name: "App-Name", Value: hymxSchema.DataProtocol},
		{Name: "NodeVersion", Value: nodeSchema.NodeVersion},
		{Name: "Variant", Value: hymxSchema.Variant},
		{Name: "Action", Value: "Upload"},
	})
	if err != nil {
		return "", err
	}
	return tx.ID, nil
}

func (o *OptGoar) Download(itemID string) (*goarSchema.BundleItem, error) {
	parentTxID, err := o.GetBundledInId(itemID)
	if err != nil {
		return nil, err
	}
	item, err := o.wallet.Client.GetBundleItems(parentTxID, []string{itemID})
	if err != nil {
		return nil, err
	}
	if len(item) == 0 {
		return nil, fmt.Errorf("download failed")
	}
	return item[0], nil
}

func (o *OptGoar) Downloads(itemsIds []string) (items []*goarSchema.BundleItem, err error) {
	if len(itemsIds) == 0 {
		return []*goarSchema.BundleItem{}, nil
	}

	// Map to group itemIDs by their parentTxid
	parentToItems := make(map[string][]string)

	// Get parentTxid for each itemID
	for _, itemID := range itemsIds {
		parentTxID, err := o.GetBundledInId(itemID)
		if err != nil {
			return nil, fmt.Errorf("failed to get parent txid for item %s: %w", itemID, err)
		}
		parentToItems[parentTxID] = append(parentToItems[parentTxID], itemID)
	}

	// Download items grouped by parentTxid
	var allItems []*goarSchema.BundleItem
	for parentTxID, itemIDs := range parentToItems {
		bundleItems, err := o.wallet.Client.GetBundleItems(parentTxID, itemIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to get bundle items for parent %s: %w", parentTxID, err)
		}
		allItems = append(allItems, bundleItems...)
	}

	return allItems, nil
}

func (o *OptGoar) GraphQL(query string) ([]byte, error) {
	return o.wallet.Client.GraphQL(query)
}

func (o *OptGoar) CheckTransaction(txid string) (bool, error) {
	state, err := o.wallet.Client.GetTransactionStatus(txid)
	if err != nil {
		return false, err
	}
	if state.NumberOfConfirmations <= 1 {
		return false, nil
	}
	fmt.Printf("txid: %s, state: %#v\n", txid, state)

	// check data
	itemBytes, err := o.wallet.Client.GetTransactionData(txid, "json")
	if err != nil {
		return false, err
	}

	_, err = goarUtils.DecodeBundle(itemBytes)
	if err != nil {
		return false, err
	}

	return true, nil
}
