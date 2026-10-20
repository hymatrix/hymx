package node

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hymatrix/hymx/db/cache"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/sdk"
	sdkSchema "github.com/hymatrix/hymx/sdk/schema"
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

func (suite *NodeForkTestSuite) TestRefreshKeepsBootstrapWithoutAlternative() {
	for _, nodes := range [][]registrySchema.Node{
		nil,
		{
			{AccId: "local", URL: "http://self"},
			{AccId: "empty"},
		},
	} {
		n, _ := suite.newForkNode(nodes...)
		client := sdk.NewClient("http://bootstrap")
		suite.T().Cleanup(client.Close)
		clients := []sdk.Client{*client}
		got := n.refreshForkClients("process", clients, "http://bootstrap")
		require.Len(suite.T(), got, 1)
		assert.Same(suite.T(), &clients[0], &got[0])
	}
}

func (suite *NodeForkTestSuite) TestRefreshKeepsAllRemoteCandidates() {
	nodes := []registrySchema.Node{
		{AccId: "local", URL: "http://self"},
		{AccId: "empty"},
	}
	for _, id := range []string{"first", "second"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.NoError(suite.T(), json.NewEncoder(w).Encode(goarSchema.BundleItem{Id: id}))
		}))
		suite.T().Cleanup(server.Close)
		nodes = append(nodes, registrySchema.Node{AccId: id, URL: server.URL})
	}
	n, _ := suite.newForkNode(nodes...)
	bootstrap := sdk.NewClient("http://bootstrap")
	suite.T().Cleanup(bootstrap.Close)
	clients := n.refreshForkClients("process", []sdk.Client{*bootstrap}, "")
	suite.T().Cleanup(func() {
		for _, client := range clients {
			client.Close()
		}
	})
	require.Len(suite.T(), clients, 2)
	ids := make([]string, 0, len(clients))
	for _, client := range clients {
		item, err := client.GetMessageByNonce("process", 0)
		require.NoError(suite.T(), err)
		ids = append(ids, item.Id)
	}
	assert.ElementsMatch(suite.T(), []string{"first", "second"}, ids)
}

func (suite *NodeForkTestSuite) TestRefreshReusesClientsAndOrder() {
	n, db := suite.newForkNode(
		registrySchema.Node{AccId: "first", URL: "http://first"},
		registrySchema.Node{AccId: "second", URL: "http://second"},
		registrySchema.Node{AccId: "duplicate", URL: "http://first"},
	)
	clients := []sdk.Client{*sdk.NewClient("http://second"), *sdk.NewClient("http://first")}
	suite.T().Cleanup(func() {
		for _, client := range clients {
			client.Close()
		}
	})
	original := clients
	for i := 0; i < 10; i++ {
		clients = n.refreshForkClients("process", clients, "")
		require.Len(suite.T(), clients, 2)
		assert.Same(suite.T(), &original[0], &clients[0])
		assert.Equal(suite.T(), "http://second", clients[0].BaseURL())
	}

	require.NoError(suite.T(), db.Register(registrySchema.Node{AccId: "second", URL: "http://new"}))
	data, err := db.Checkpoint()
	require.NoError(suite.T(), err)
	vm, _, err := n.vmm.GetVm("registry")
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), vm.Restore(data))
	clients = n.refreshForkClients("process", clients, "")
	require.Len(suite.T(), clients, 2)
	assert.Equal(suite.T(), original[1], clients[0])
	assert.Equal(suite.T(), "http://new", clients[1].BaseURL())
}

func (suite *NodeForkTestSuite) TestNotFoundKeepsExplicitSourceAndConnection() {
	for _, path := range []string{"/messageByNonce/process/7", "/assignmentByNonce/process/7"} {
		suite.Run(path, func() {
			var connections, alternateRequests atomic.Int32
			source := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == path {
					http.Error(w, "next nonce is not available", http.StatusNotFound)
				} else {
					assert.NoError(suite.T(), json.NewEncoder(w).Encode(forkTestMessage()))
				}
			}))
			source.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateNew {
					connections.Add(1)
				}
			}
			source.Start()
			suite.T().Cleanup(source.Close)
			alternate := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				alternateRequests.Add(1)
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			suite.T().Cleanup(alternate.Close)
			n, _ := suite.newForkNode(registrySchema.Node{AccId: "stale", URL: alternate.URL})
			clients := []sdk.Client{*sdk.NewClient(source.URL)}
			suite.T().Cleanup(clients[0].Close)
			for i := 0; i < 4; i++ {
				_, err := clients[0].GetMessageByNonce("process", 7)
				if err == nil {
					_, err = clients[0].GetAssignByNonce("process", 7)
				}
				got := n.retryForkClients("process", clients, source.URL, err)
				var status *sdkSchema.HTTPStatusError
				require.True(suite.T(), errors.As(err, &status))
				assert.Equal(suite.T(), http.StatusNotFound, status.StatusCode)
				assert.Equal(suite.T(), source.URL, got[0].BaseURL())
			}
			assert.EqualValues(suite.T(), 1, connections.Load())
			assert.Zero(suite.T(), alternateRequests.Load())
		})
	}
}

func (suite *NodeForkTestSuite) TestRetryRetainsConfiguredFallback() {
	n, _ := suite.newForkNode(registrySchema.Node{AccId: "remote", URL: "http://new"})
	clients := []sdk.Client{*sdk.NewClient("http://configured")}
	suite.T().Cleanup(func() {
		for _, client := range clients {
			client.Close()
		}
	})
	clients = n.retryForkClients("process", clients, "http://configured",
		&url.Error{Op: "Get", URL: "http://configured", Err: errors.New("connection refused")})
	require.Len(suite.T(), clients, 2)
	assert.Equal(suite.T(), "http://new", clients[0].BaseURL())
	assert.Equal(suite.T(), "http://configured", clients[1].BaseURL())

	clients = n.retryForkClients("process", clients, "http://configured",
		&sdkSchema.HTTPStatusError{StatusCode: http.StatusServiceUnavailable})
	assert.Equal(suite.T(), "http://configured", clients[0].BaseURL())
}

func (suite *NodeForkTestSuite) TestNotFoundDropsStaleRegistryURL() {
	n, _ := suite.newForkNode(registrySchema.Node{AccId: "remote", URL: "http://new"})
	clients := []sdk.Client{*sdk.NewClient("http://old")}
	suite.T().Cleanup(func() {
		for _, client := range clients {
			client.Close()
		}
	})
	clients = n.retryForkClients("process", clients, "",
		&sdkSchema.HTTPStatusError{StatusCode: http.StatusNotFound})
	require.Len(suite.T(), clients, 1)
	assert.Equal(suite.T(), "http://new", clients[0].BaseURL())
}

func (suite *NodeForkTestSuite) TestForkRefreshesAfterReadFailure() {
	for _, failedPath := range []string{"/messageByNonce/process/0", "/assignmentByNonce/process/0"} {
		suite.Run(failedPath, func() {
			entered := make(chan struct{})
			release := make(chan struct{})
			var enteredOnce, releaseOnce sync.Once
			old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == failedPath {
					enteredOnce.Do(func() { close(entered) })
					<-release
					w.WriteHeader(http.StatusServiceUnavailable)
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
			go func() {
				defer close(done)
				n.runForkProcess("process", "", []sdk.Client{*sdk.NewClient(old.URL)}, old.URL, vmmSchema.ExecModeDryRun)
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
			data, err := db.Checkpoint()
			require.NoError(suite.T(), err)
			vm, _, err := n.vmm.GetVm("registry")
			require.NoError(suite.T(), err)
			require.NoError(suite.T(), vm.Restore(data))
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
	data, err := db.Checkpoint()
	require.NoError(suite.T(), err)
	vm, _, err := n.vmm.GetVm("registry")
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), vm.Restore(data))
	require.NoError(suite.T(), n.Fork("process", "", "", vmmSchema.ExecModeDryRun))
	select {
	case <-requested:
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "registry endpoint was not requested")
	}
	done := make(chan struct{})
	go func() {
		n.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "fork did not exit")
	}
}

func TestNodeForkTestSuite(t *testing.T) {
	suite.Run(t, new(NodeForkTestSuite))
}
