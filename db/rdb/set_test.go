package rdb

import (
	"context"
	"strconv"
	"testing"

	"github.com/hymatrix/hymx/db/rdb/schema"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestGetAllProcess(t *testing.T) {
	// Create a Redis client for testing
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use DB 1 for testing to avoid affecting other data
	})
	defer rdb.FlushDB(context.Background())
	defer rdb.Close()

	r := &RDB{
		rdb: rdb,
		ctx: context.Background(),
	}

	// Prepare test data
	testData := map[string]string{
		"process1": "10",
		"process2": "20",
		"process3": "30",
	}

	// Write test data
	for pid, nonce := range testData {
		err := rdb.Set(r.ctx, schema.RdbProcessNoncePrefix+pid, nonce, 0).Err()
		assert.NoError(t, err)
	}

	// Test normal case
	t.Run("Normal case", func(t *testing.T) {
		pids, nonces, err := r.GetAllProcess()
		assert.NoError(t, err)
		assert.Equal(t, len(testData), len(pids))
		assert.Equal(t, len(testData), len(nonces))

		// Verify if the returned data is correct
		for i, pid := range pids {
			expectedNonce := testData[pid]
			expectedIntNonce, err := strconv.ParseInt(expectedNonce, 10, 64)
			assert.NoError(t, err)
			assert.NotEmpty(t, expectedNonce)
			assert.Equal(t, expectedIntNonce, nonces[i])
		}
	})

	// Test empty data case
	t.Run("Empty case", func(t *testing.T) {
		// Clear database
		err := rdb.FlushDB(r.ctx).Err()
		assert.NoError(t, err)

		pids, nonces, err := r.GetAllProcess()
		assert.NoError(t, err)
		assert.Empty(t, pids)
		assert.Empty(t, nonces)
	})

	// Test invalid data case
	t.Run("Invalid data case", func(t *testing.T) {
		// Write an invalid nonce value
		err := rdb.Set(r.ctx, schema.RdbProcessNoncePrefix+"invalid", "not_a_number", 0).Err()
		assert.NoError(t, err)

		pids, nonces, err := r.GetAllProcess()
		assert.NoError(t, err)
		// Since invalid data will be skipped, the result should be empty
		assert.Empty(t, pids)
		assert.Empty(t, nonces)
	})
}
