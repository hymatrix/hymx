package chainkit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hymatrix/hymx/chainkit/schema"
)

const (
	queryBatchSize = 10
)

// query assignment & message by nonce
func (c *Chainkit) queryByNonce(scheduler, pid string, beginNonce, endNonce int64) (assignIds, txIds map[int64]string, err error) {
	// Query GraphQL for assignment messages, assignment message tags contain message txid
	assignIds = make(map[int64]string)
	txIds = make(map[int64]string)

	// batch download
	for currentBegin := beginNonce; currentBegin <= endNonce; currentBegin += queryBatchSize {
		// Calculate end nonce for current batch
		currentEnd := currentBegin + queryBatchSize - 1
		if currentEnd > endNonce {
			currentEnd = endNonce
		}

		// Calculate size of current batch
		currentSize := currentEnd - currentBegin + 1

		// Call queryBatch to process current batch
		batchAssignIds, batchTxIds, err := c.queryBatch(scheduler, pid, currentBegin, currentSize)
		if err != nil {
			return nil, nil, fmt.Errorf("query batch failed for nonce range %d-%d: %w", currentBegin, currentEnd, err)
		}

		// Merge results
		for nonce, assignId := range batchAssignIds {
			assignIds[nonce] = assignId
		}
		for nonce, txId := range batchTxIds {
			txIds[nonce] = txId
		}
	}

	return assignIds, txIds, nil
}

func (c *Chainkit) queryBatch(scheduler, pid string, beginNonce, size int64) (assignIds, txIds map[int64]string, err error) {
	// Query conditions:
	// Owner = scheduler
	// tag: Nonce = beginNonce to beginNonce+size-1
	// tag: Process = pid
	// Get results:
	// Id (Assignment)
	// tag: Message

	assignIds = make(map[int64]string)
	txIds = make(map[int64]string)

	endNonce := beginNonce + size - 1

	// Generate nonce value list
	var nonceValues []string
	for i := beginNonce; i <= endNonce; i++ {
		nonceValues = append(nonceValues, fmt.Sprintf(`"%d"`, i))
	}
	nonceValuesStr := strings.Join(nonceValues, ", ")

	query := fmt.Sprintf(schema.QueryTmp, scheduler, pid, nonceValuesStr)
	log.Debug("queryql", "query", query)

	response, err := c.Query(query)
	if err != nil {
		return nil, nil, fmt.Errorf("GraphQL query failed: %w", err)
	}

	var graphQLResp schema.GraphQLResp
	if err := json.Unmarshal(response, &graphQLResp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	log.Debug("graphql response", "response", graphQLResp)

	// Iterate through edges array
	for _, edge := range graphQLResp.Transactions.Edges {
		node := edge.Node

		// Get assignment ID (node.id)
		assignId := node.ID
		if assignId == "" {
			continue
		}

		var nonceStr, messageId string
		// Iterate through tags to find Nonce and Message
		for _, tag := range node.Tags {
			switch tag.Name {
			case "Nonce":
				nonceStr = tag.Value
			case "Message":
				messageId = tag.Value
			}
		}

		// If valid nonce found, add to results
		if nonceStr != "" {
			if nonce, err := strconv.ParseInt(nonceStr, 10, 64); err == nil {
				assignIds[nonce] = assignId
				if messageId != "" {
					txIds[nonce] = messageId
				}
			}
		}
	}

	return assignIds, txIds, nil
}
