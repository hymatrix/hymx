package optgoar

import (
	"fmt"

	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func Download(itemID string, client *goar.Client) (*goarSchema.BundleItem, error) {
	parentTxID, err := GetBundledInId(itemID, client)
	if err != nil {
		return nil, err
	}
	item, err := client.GetBundleItems(parentTxID, []string{itemID})
	if err != nil {
		return nil, err
	}
	if len(item) == 0 {
		return nil, fmt.Errorf("download failed")
	}
	return item[0], nil
}

func Downloads(itemsIds []string, client *goar.Client) (items []*goarSchema.BundleItem, err error) {
	if len(itemsIds) == 0 {
		return []*goarSchema.BundleItem{}, nil
	}

	// Map to group itemIDs by their parentTxid
	parentToItems := make(map[string][]string)

	// Get parentTxid for each itemID
	for _, itemID := range itemsIds {
		parentTxID, err := GetBundledInId(itemID, client)
		if err != nil {
			return nil, fmt.Errorf("failed to get parent txid for item %s: %w", itemID, err)
		}
		parentToItems[parentTxID] = append(parentToItems[parentTxID], itemID)
	}

	// Download items grouped by parentTxid
	var allItems []*goarSchema.BundleItem
	for parentTxID, itemIDs := range parentToItems {
		bundleItems, err := client.GetBundleItems(parentTxID, itemIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to get bundle items for parent %s: %w", parentTxID, err)
		}
		allItems = append(allItems, bundleItems...)
	}

	return allItems, nil
}

func GraphQL(query string, client *goar.Client) ([]byte, error) {
	return client.GraphQL(query)
}

func CheckTransaction(txid string, client *goar.Client) (bool, error) {
	state, err := client.GetTransactionStatus(txid)
	if err != nil {
		return false, err
	}
	if state.NumberOfConfirmations <= 1 {
		return false, nil
	}

	// check data
	itemBytes, err := client.GetTransactionData(txid, "json")
	if err != nil {
		return false, err
	}

	_, err = goarUtils.DecodeBundle(itemBytes)
	if err != nil {
		return false, err
	}

	return true, nil
}
