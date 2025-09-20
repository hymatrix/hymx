package optgoar

import (
	"encoding/json"
	"fmt"
)

const (
	BundledInQueryTemplate = `{
		transaction(id: "%s") {
			bundledIn {
				id
			}
		}
	}`
)

// BundledInResponse GraphQL response structure
type BundledInResponse struct {
	Transaction struct {
		BundledIn struct {
			ID string `json:"id"`
		} `json:"bundledIn"`
	} `json:"transaction"`
}

func (o *OptGoar) GetBundledInId(txid string) (parentTxid string, err error) {
	query := fmt.Sprintf(BundledInQueryTemplate, txid)
	result, err := o.GraphQL(query)
	if err != nil {
		return
	}

	parentTxid, err = o.parseBundledInID(string(result))
	if err != nil {
		return "", fmt.Errorf("failed to parse parent txid: %w", err)
	}

	return parentTxid, nil
}

func (o *OptGoar) parseBundledInID(jsonStr string) (string, error) {
	var response BundledInResponse
	fmt.Println(jsonStr)
	err := json.Unmarshal([]byte(jsonStr), &response)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	if response.Transaction.BundledIn.ID == "" {
		return "", fmt.Errorf("bundledIn.id not found or empty")
	}

	return response.Transaction.BundledIn.ID, nil
}
