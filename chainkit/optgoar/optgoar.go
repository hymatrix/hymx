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

func (o *OptGoar) Download(parentTxID string, itemsIds []string) (items []*goarSchema.BundleItem, err error) {
	return o.wallet.Client.GetBundleItems(parentTxID, itemsIds)
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
