package rdb

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/hymatrix/hymx/db/rdb/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startOutboxRedis runs an isolated Redis instance for integration tests.
func startOutboxRedis(t *testing.T) string {
	t.Helper()
	redisPath, err := exec.LookPath("redis-server")
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	_, port, err := net.SplitHostPort(address)
	require.NoError(t, err)
	work := t.TempDir()
	logFile, err := os.Create(filepath.Join(work, "redis.log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = logFile.Close() })
	command := exec.Command(redisPath, "--bind", "127.0.0.1", "--port", port,
		"--save", "", "--appendonly", "no", "--dir", work)
	command.Stdout, command.Stderr = logFile, logFile
	require.NoError(t, command.Start())
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	url := "redis://" + address
	r := New(url)
	t.Cleanup(r.Close)
	require.Eventually(t, func() bool {
		return r.rdb.Ping(r.ctx).Err() == nil
	}, 5*time.Second, 10*time.Millisecond)
	return url
}

func TestOutboxIntegration(t *testing.T) {
	if os.Getenv("HYMX_INTEGRATION") != "1" {
		t.Skip("set HYMX_INTEGRATION=1 to run isolated Redis integration tests")
	}
	url := startOutboxRedis(t)
	r := New(url)
	t.Cleanup(r.Close)

	t.Run("FIFOAndClientReconnect", func(t *testing.T) {
		first := goarSchema.BundleItem{
			Id: "first", Target: "target", Data: "aGVsbG8=",
			Owner: "owner", Signature: "signature", SignatureType: 1,
			Tags: []goarSchema.Tag{{Name: "Sequence", Value: "1"}},
		}
		second := goarSchema.BundleItem{Id: "second", Data: "d29ybGQ="}
		writer := New(url)
		t.Cleanup(writer.Close)
		require.NoError(t, writer.PushOutbox("sender", "target", first))
		require.NoError(t, writer.PushOutbox("sender", "target", second))
		writer.Close()
		reader := New(url)
		t.Cleanup(reader.Close)

		for i := 0; i < 2; i++ {
			message, err := reader.PeekOutbox("sender", "target")
			require.NoError(t, err)
			assert.Equal(t, &first, message)
		}
		require.NoError(t, reader.CommitOutbox("sender", "target"))
		message, err := reader.PeekOutbox("sender", "target")
		require.NoError(t, err)
		assert.Equal(t, &second, message)
		require.NoError(t, reader.CommitOutbox("sender", "target"))
		message, err = reader.PeekOutbox("sender", "target")
		require.NoError(t, err)
		assert.Nil(t, message)
		assert.Error(t, reader.CommitOutbox("sender", "target"))
		exists, err := r.rdb.Exists(r.ctx, schema.RdbOutboxPrefix+"sender:target").Result()
		require.NoError(t, err)
		assert.Zero(t, exists)
	})

	t.Run("QueueIsolation", func(t *testing.T) {
		queues := []struct{ pid, target, id string }{
			{pid: "a", target: "x", id: "ax"},
			{pid: "a", target: "y", id: "ay"},
			{pid: "b", target: "x", id: "bx"},
		}
		for _, queue := range queues {
			require.NoError(t, r.PushOutbox(queue.pid, queue.target, goarSchema.BundleItem{Id: queue.id}))
		}
		for _, queue := range queues {
			message, err := r.PeekOutbox(queue.pid, queue.target)
			require.NoError(t, err)
			require.NotNil(t, message)
			assert.Equal(t, queue.id, message.Id)
			require.NoError(t, r.CommitOutbox(queue.pid, queue.target))
		}
	})

	t.Run("MissingQueue", func(t *testing.T) {
		message, err := r.PeekOutbox("missing", "target")
		require.NoError(t, err)
		assert.Nil(t, message)
		assert.Error(t, r.CommitOutbox("missing", "target"))
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		require.NoError(t, r.rdb.RPush(r.ctx, schema.RdbOutboxPrefix+"invalid:target", "{").Err())
		message, err := r.PeekOutbox("invalid", "target")
		assert.Error(t, err)
		assert.Nil(t, message)
	})

	t.Run("RedisErrors", func(t *testing.T) {
		require.NoError(t, r.rdb.Set(r.ctx, schema.RdbOutboxPrefix+"wrong:target", "not a list", 0).Err())
		assert.Error(t, r.PushOutbox("wrong", "target", goarSchema.BundleItem{Id: "message"}))
		message, err := r.PeekOutbox("wrong", "target")
		assert.Error(t, err)
		assert.Nil(t, message)
		assert.Error(t, r.CommitOutbox("wrong", "target"))
	})
}
