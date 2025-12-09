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
	if len(items) == 0 {
		return "", fmt.Errorf("items is empty")
	}

	// gen binary
	for i := range items {
		if len(items[i].Binary) == 0 {
			bin, err1 := goarUtils.GenerateItemBinary(items[i])
			if err1 != nil {
				return "", fmt.Errorf("failed to generate item binary, bundleId: %s, err: %w", items[i].Id, err)
			}
			items[i].Binary = bin
		}
	}

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
	return Download(itemID, o.wallet.Client)
}

func (o *OptGoar) Downloads(itemsIds []string) (items []*goarSchema.BundleItem, err error) {
	return Downloads(itemsIds, o.wallet.Client)
}

func (o *OptGoar) GraphQL(query string) (result []byte, err error) {
	return GraphQL(query, o.wallet.Client)
}

func (o *OptGoar) CheckTransaction(txid string) (bool, error) {
	return CheckTransaction(txid, o.wallet.Client)
}
