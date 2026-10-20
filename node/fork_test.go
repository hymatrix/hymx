package node

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hymatrix/hymx/db/cache"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/hymatrix/hymx/vmm"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NodeForkTestSuite struct {
	suite.Suite
}

func (suite *NodeForkTestSuite) newForkNode(nodes ...registrySchema.Node) (*Node, *cache.Registry) {
	ctx, cancel := context.WithCancel(context.Background())
	n := &Node{
		info:   &nodeSchema.Info{Node: registrySchema.Node{AccId: "local"}},
		ctx:    ctx,
		cancel: cancel,
	}
	n.vmm = vmm.New(nil, n.info, make(chan vmmSchema.VmmResult, 10),
		make(chan vmmSchema.Outbox, 10), make(chan struct{}, 1))
	n.vmm.Run()
	suite.T().Cleanup(n.vmm.Close)
	suite.T().Cleanup(cancel)
	db := cache.NewRegistry("registry", "token", registrySchema.Node{AccId: "main"})
	for _, node := range nodes {
		require.NoError(suite.T(), db.Register(node))
		require.NoError(suite.T(), db.RegisterProcess(node.AccId, "process"))
	}
	data, err := db.Checkpoint()
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), n.vmm.Restore(vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta: vmmSchema.Meta{
				Pid: "registry", AccId: "main",
				Params: map[string]string{"Token-Pid": "token"},
			},
			Module: hymxSchema.Module{ModuleFormat: vmmSchema.ModuleFormatRegistry},
		},
		Data: data,
	}))
	return n, db
}

func forkTestMessage() goarSchema.BundleItem {
	return goarSchema.BundleItem{
		SignatureType: goarSchema.ArweaveSignType,
		Owner:         "AA",
		Target:        "process",
		Tags:          []goarSchema.Tag{{Name: "Type", Value: hymxSchema.TypeMessage}},
	}
}

func forkTestAssignment() goarSchema.BundleItem {
	return goarSchema.BundleItem{Tags: []goarSchema.Tag{
		{Name: "Nonce", Value: "0"},
		{Name: "Timestamp", Value: "1"},
	}}
}

func (suite *NodeForkTestSuite) publishRegistry(n *Node, db *cache.Registry) {
	data, err := db.Checkpoint()
	require.NoError(suite.T(), err)
	vm, _, err := n.vmm.GetVm("registry")
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), vm.Restore(data))
}

func (suite *NodeForkTestSuite) TestRefreshReusesClientUntilURLChanges() {
	n, db := suite.newForkNode(registrySchema.Node{AccId: "remote", URL: "http://old"})
	url, client, err := n.refreshForkClient("process", "", nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), client)
	suite.T().Cleanup(func() { client.Close() })
	original := client
	url, client, err = n.refreshForkClient("process", url, client)
	require.NoError(suite.T(), err)
	assert.Same(suite.T(), original, client)
	require.NoError(suite.T(), db.Unregister("remote"))
	suite.publishRegistry(n, db)
	url, client, err = n.refreshForkClient("process", url, client)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "http://old", url)
	assert.Same(suite.T(), original, client)
	require.NoError(suite.T(), db.Register(registrySchema.Node{AccId: "remote", URL: "http://new"}))
	suite.publishRegistry(n, db)
	url, client, err = n.refreshForkClient("process", url, client)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "http://new", url)
	assert.NotSame(suite.T(), original, client)
}

func (suite *NodeForkTestSuite) TestForkRejectsMissingRemoteNode() {
	for _, node := range []registrySchema.Node{
		{AccId: "local", URL: "http://self"},
		{AccId: "empty"},
	} {
		n, _ := suite.newForkNode(node)
		assert.ErrorIs(suite.T(), n.Fork("process", "", "", vmmSchema.ExecModeDryRun), nodeSchema.ErrNotFoundNodes)
	}
}

func (suite *NodeForkTestSuite) TestRefreshClosesRemovedClientConnection() {
	closed := make(chan struct{})
	var once sync.Once
	source := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(suite.T(), json.NewEncoder(w).Encode(forkTestMessage()))
	}))
	source.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			once.Do(func() { close(closed) })
		}
	}
	source.Start()
	suite.T().Cleanup(source.Close)
	n, _ := suite.newForkNode(registrySchema.Node{AccId: "remote", URL: "http://new"})
	client := sdk.NewClient(source.URL)
	suite.T().Cleanup(client.Close)
	_, err := client.GetMessageByNonce("process", 0)
	require.NoError(suite.T(), err)
	_, replacement, err := n.refreshForkClient("process", source.URL, client)
	require.NoError(suite.T(), err)
	suite.T().Cleanup(replacement.Close)
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "removed client retained its idle connection")
	}
}

func (suite *NodeForkTestSuite) TestExplicitUnregisteredGatewayKeepsSourceOnNotFound() {
	for _, failedPath := range []string{"/messageByNonce/process/0", "/assignmentByNonce/process/0"} {
		suite.Run(failedPath, func() {
			var alternateRequests, failures atomic.Int32
			alternate := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				alternateRequests.Add(1)
				w.WriteHeader(http.StatusNotFound)
			}))
			suite.T().Cleanup(alternate.Close)
			n, _ := suite.newForkNode(registrySchema.Node{AccId: "remote", URL: alternate.URL})
			repeated := make(chan struct{})
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == failedPath {
					if failures.Add(1) == 2 {
						n.cancel()
						close(repeated)
					}
					w.WriteHeader(http.StatusNotFound)
					return
				}
				assert.NoError(suite.T(), json.NewEncoder(w).Encode(forkTestMessage()))
			}))
			suite.T().Cleanup(gateway.Close)
			require.NoError(suite.T(), n.Fork("process", "", gateway.URL, vmmSchema.ExecModeDryRun))
			done := make(chan struct{})
			go func() { n.wg.Wait(); close(done) }()
			suite.T().Cleanup(func() {
				n.cancel()
				select {
				case <-done:
				case <-time.After(15 * time.Second):
					assert.Fail(suite.T(), "fork did not exit")
				}
			})
			select {
			case <-repeated:
			case <-time.After(15 * time.Second):
				require.FailNow(suite.T(), "gateway did not receive repeated polls")
			}
			assert.Zero(suite.T(), alternateRequests.Load())
		})
	}
}

func (suite *NodeForkTestSuite) TestForkRefreshesAfterReadFailure() {
	for _, failure := range []struct {
		name       string
		path       string
		disconnect bool
	}{
		{name: "message404", path: "/messageByNonce/process/0"},
		{name: "assignment404", path: "/assignmentByNonce/process/0"},
		{name: "messageDisconnect", path: "/messageByNonce/process/0", disconnect: true},
		{name: "assignmentDisconnect", path: "/assignmentByNonce/process/0", disconnect: true},
	} {
		suite.Run(failure.name, func() {
			entered := make(chan struct{})
			release := make(chan struct{})
			var enteredOnce, releaseOnce sync.Once
			old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == failure.path {
					enteredOnce.Do(func() { close(entered) })
					<-release
					if failure.disconnect {
						conn, _, err := w.(http.Hijacker).Hijack()
						if assert.NoError(suite.T(), err) {
							conn.Close()
						}
						return
					}
					w.WriteHeader(http.StatusNotFound)
					return
				}
				assert.NoError(suite.T(), json.NewEncoder(w).Encode(forkTestMessage()))
			}))
			suite.T().Cleanup(old.Close)
			n, db := suite.newForkNode(registrySchema.Node{AccId: "remote", URL: old.URL})
			requests := make(chan string, 4)
			current := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case requests <- r.URL.Path:
				case <-n.ctx.Done():
					return
				}
				if r.URL.Path == "/assignmentByNonce/process/0" {
					n.cancel()
					assert.NoError(suite.T(), json.NewEncoder(w).Encode(forkTestAssignment()))
					return
				}
				assert.NoError(suite.T(), json.NewEncoder(w).Encode(forkTestMessage()))
			}))
			suite.T().Cleanup(current.Close)
			done := make(chan struct{})
			n.wg.Add(1)
			go func() {
				defer close(done)
				n.runForkProcess("process", "", true, old.URL, sdk.NewClient(old.URL), vmmSchema.ExecModeDryRun)
			}()
			suite.T().Cleanup(func() {
				n.cancel()
				releaseOnce.Do(func() { close(release) })
				select {
				case <-done:
				case <-time.After(25 * time.Second):
					assert.Fail(suite.T(), "fork did not exit")
				}
			})
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				require.FailNow(suite.T(), "old endpoint was not requested")
			}

			// Publish updated registry state while the old request is blocked.
			require.NoError(suite.T(), db.Register(registrySchema.Node{AccId: "remote", URL: current.URL}))
			suite.publishRegistry(n, db)
			releaseOnce.Do(func() { close(release) })

			for _, want := range []string{"/messageByNonce/process/0", "/assignmentByNonce/process/0"} {
				select {
				case got := <-requests:
					assert.Equal(suite.T(), want, got)
				case <-time.After(15 * time.Second):
					require.FailNow(suite.T(), "new endpoint was not requested")
				}
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				require.FailNow(suite.T(), "fork did not exit after successful retry")
			}
		})
	}
}

func (suite *NodeForkTestSuite) TestForkSelectsRegistryURL() {
	n, db := suite.newForkNode()
	requested := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(requested) })
		n.cancel()
		if r.URL.Path == "/assignmentByNonce/process/0" {
			assert.NoError(suite.T(), json.NewEncoder(w).Encode(forkTestAssignment()))
		} else {
			assert.NoError(suite.T(), json.NewEncoder(w).Encode(forkTestMessage()))
		}
	}))
	suite.T().Cleanup(server.Close)
	require.NoError(suite.T(), db.Register(registrySchema.Node{AccId: "remote", URL: server.URL}))
	require.NoError(suite.T(), db.RegisterProcess("remote", "process"))
	suite.publishRegistry(n, db)
	require.NoError(suite.T(), n.Fork("process", "", "", vmmSchema.ExecModeDryRun))
	done := make(chan struct{})
	go func() {
		n.wg.Wait()
		close(done)
	}()
	suite.T().Cleanup(func() {
		n.cancel()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			assert.Fail(suite.T(), "fork did not exit")
		}
	})
	select {
	case <-requested:
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "registry endpoint was not requested")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "fork did not exit")
	}
}

func TestNodeForkTestSuite(t *testing.T) {
	suite.Run(t, new(NodeForkTestSuite))
}
