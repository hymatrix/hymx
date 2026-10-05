package node

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	cacheSchema "github.com/hymatrix/hymx/db/cache/schema"
	"github.com/hymatrix/hymx/db/rdb"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar"
	goarUtils "github.com/permadao/goar/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OutboxIntegrationTestSuite struct {
	suite.Suite
}

func (suite *OutboxIntegrationTestSuite) TestRecreatedNodeSendsStoredOutbox() {
	t := suite.T()
	keyfile, err := filepath.Abs("../cmd/test_keyfile.json")
	require.NoError(t, err)
	signer, err := goar.NewSignerFromPath(keyfile)
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	work := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(work))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })
	redisPath, err := exec.LookPath("redis-server")
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	_, port, err := net.SplitHostPort(address)
	require.NoError(t, err)
	logFile, err := os.Create(filepath.Join(work, "redis.log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = logFile.Close() })
	command := exec.Command(redisPath, "--bind", "127.0.0.1", "--port", port,
		"--save", "", "--appendonly", "no", "--dir", work)
	command.Stdout, command.Stderr = logFile, logFile
	require.NoError(t, command.Start())
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	redisURL := "redis://" + address
	db := rdb.New(redisURL)
	t.Cleanup(db.Close)
	require.Eventually(t, func() bool {
		_, err := db.PeekOutbox("sender", "target")
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)

	tags, err := utils.MessageToTags(hymxSchema.Message{Base: hymxSchema.DefaultBaseMessage, Action: "Info"})
	require.NoError(t, err)
	item, err := bundler.CreateAndSignItem([]byte("pending"), bundler.Address, "", tags)
	require.NoError(t, err)
	first := New(nil, bundler, redisURL, "", "", &nodeSchema.Info{}, nil)
	t.Cleanup(first.db.(*rdb.RDB).Close)
	t.Cleanup(first.sdk.Close)
	closeFirst := sync.OnceFunc(first.Close)
	t.Cleanup(closeFirst)
	require.NoError(t, first.db.PushOutbox("sender", item.Target, item))
	closeFirst()
	first.db.(*rdb.RDB).Close()

	_, assignment, err := first.signAssign(item.Target, item.Id, 0)
	require.NoError(t, err)
	assignmentData, err := json.Marshal(assignment)
	require.NoError(t, err)
	received := make(chan []byte, 1)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/":
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			select {
			case received <- data:
			default:
			}
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/assignmentByMessage/"+item.Id:
			_, _ = w.Write(assignmentData)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(remote.Close)
	n := New(nil, bundler, redisURL, "", remote.URL,
		&nodeSchema.Info{Node: registrySchema.Node{AccId: "local-node"}}, nil)
	t.Cleanup(n.db.(*rdb.RDB).Close)
	t.Cleanup(n.sdk.Close)
	t.Cleanup(n.Close)
	n.vmm.Run()
	remoteNode := &registrySchema.Node{AccId: bundler.Address, URL: remote.URL}
	registryData, err := json.Marshal(cacheSchema.RegistrySnapshot{
		Id: "registry-pid", TokenPid: "token-pid",
		Registered: map[string]bool{bundler.Address: true},
		ProcessToNodeIndex: map[string]map[string]*registrySchema.Node{
			item.Target: {bundler.Address: remoteNode},
		},
	})
	require.NoError(t, err)
	require.NoError(t, n.vmm.Restore(vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta: vmmSchema.Meta{
				Pid: "registry-pid", ItemId: "registry-pid", AccId: bundler.Address,
				Params: map[string]string{"Token-Pid": "token-pid", "URL": remote.URL},
			},
			Module: hymxSchema.Module{ModuleFormat: vmmSchema.ModuleFormatRegistry},
		},
		Data: string(registryData),
	}))

	// Restoring a legacy VM snapshot must leave the current Redis queue intact.
	snapshot, err := n.vmm.Checkpoint("registry-pid")
	require.NoError(t, err)
	snapshot.Outbox = `{"i":"sender","t":{}}`
	checkpoint, err := n.signCheckpoint(snapshot)
	require.NoError(t, err)
	require.NoError(t, saveCheckpoint(checkpoint))
	_, err = n.Restore(checkpoint.Id)
	require.NoError(t, err)
	pending, err := n.db.PeekOutbox("sender", item.Target)
	require.NoError(t, err)
	require.NotNil(t, pending)
	assert.Equal(t, item.Id, pending.Id)

	// Also wait on failure paths before closing the Redis client or HTTP receiver.
	t.Cleanup(func() {
		require.Eventually(t, func() bool { return !n.isSending("sender", item.Target) },
			15*time.Second, 10*time.Millisecond)
	})
	n.TrySend("sender", item.Target)
	select {
	case data := <-received:
		expected, err := goarUtils.GenerateItemBinary(item)
		require.NoError(t, err)
		assert.Equal(t, expected, data)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for stored outbox delivery")
	}
	require.Eventually(t, func() bool {
		pending, err := n.db.PeekOutbox("sender", item.Target)
		return err == nil && pending == nil && !n.isSending("sender", item.Target)
	}, 10*time.Second, 10*time.Millisecond)
}

func TestOutboxIntegrationTestSuite(t *testing.T) {
	if os.Getenv("HYMX_INTEGRATION") != "1" {
		t.Skip("set HYMX_INTEGRATION=1 to run with isolated local Redis")
	}
	suite.Run(t, new(OutboxIntegrationTestSuite))
}
