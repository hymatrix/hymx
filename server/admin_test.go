package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/node"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/permadao/goar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// AdminTestSuite tests admin HTTP handlers.
type AdminTestSuite struct {
	suite.Suite
}

func (suite *AdminTestSuite) TestAdminStopRouteReturnsInvalidParamsWhenPidMissing() {
	s := &Server{}
	engine := newTestAdminEngine(s)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/vms/stop", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.JSONEq(suite.T(), `{"error":"err_invalid_params"}`, w.Body.String())
}

func (suite *AdminTestSuite) TestAdminResumeRouteReturnsInvalidParamsWhenPidMissing() {
	s := &Server{}
	engine := newTestAdminEngine(s)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/vms/resume", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.JSONEq(suite.T(), `{"error":"err_invalid_params"}`, w.Body.String())
}

func (suite *AdminTestSuite) TestAdminStopRouteReturnsNodeErrorEnvelopeForValidPid() {
	s := newTestServerWithNode(suite.T())
	engine := newTestAdminEngine(s)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/vms/stop", bytes.NewBufferString(`{"pid":"pid-1"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.JSONEq(suite.T(), `{"error":"err_process_not_found"}`, w.Body.String())
}

func (suite *AdminTestSuite) TestAdminResumeRouteReturnsNodeErrorEnvelopeForValidPid() {
	s := newTestServerWithNode(suite.T())
	engine := newTestAdminEngine(s)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/vms/resume", bytes.NewBufferString(`{"pid":"pid-1"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.JSONEq(suite.T(), `{"error":"err_process_not_found"}`, w.Body.String())
}

func (suite *AdminTestSuite) TestAdminRunningRouteReturnsNodePids() {
	s := newTestServerWithNode(suite.T())
	engine := newTestAdminEngine(s)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/vms/running", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	res := []string{}
	assert.NoError(suite.T(), json.Unmarshal(w.Body.Bytes(), &res))
	assert.Empty(suite.T(), res)
}

func (suite *AdminTestSuite) TestAdminRoutesAreRegistered() {
	s := &Server{}
	engine := newTestAdminEngine(s)

	routes := []string{}
	for _, route := range engine.Routes() {
		routes = append(routes, route.Method+" "+route.Path)
	}
	assert.Contains(suite.T(), routes, "POST /admin/vms/stop")
	assert.Contains(suite.T(), routes, "POST /admin/vms/resume")
	assert.Contains(suite.T(), routes, "GET /admin/vms/running")
}

func newTestAdminEngine(s *Server) *gin.Engine {
	engine := gin.Default()
	engine.Use(common.CORSMiddleware())
	engine.POST("/admin/vms/stop", s.Stop)
	engine.POST("/admin/vms/resume", s.Resume)
	engine.GET("/admin/vms/running", s.Running)
	return engine
}

func newTestServerWithNode(t *testing.T) *Server {
	t.Helper()

	keyfile, err := filepath.Abs("../cmd/test_keyfile.json")
	require.NoError(t, err)
	signer, err := goar.NewSignerFromPath(keyfile)
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)

	n := node.New(
		nil,
		bundler,
		"redis://@localhost:6379/0",
		"",
		"",
		&nodeSchema.Info{Node: registrySchema.Node{AccId: "local-node"}},
		nil,
	)
	return New(n, nil)
}

func TestAdminTestSuite(t *testing.T) {
	suite.Run(t, new(AdminTestSuite))
}
