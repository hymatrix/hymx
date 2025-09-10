package chainkit

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

func (c *Chainkit) getParentTxid(txid string) (parentTxid string, err error) {
	query := fmt.Sprintf(BundledInQueryTemplate, txid)
	result, err := c.operator.GraphQL(query)
	if err != nil {
		return
	}

	parentTxid, err = c.parseBundledInID(string(result))
	if err != nil {
		return "", fmt.Errorf("failed to parse parent txid: %w", err)
	}

	return parentTxid, nil
}

func (c *Chainkit) parseBundledInID(jsonStr string) (string, error) {
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
