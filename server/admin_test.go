package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hymatrix/hymx/common"
	"github.com/stretchr/testify/assert"
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

func TestAdminTestSuite(t *testing.T) {
	suite.Run(t, new(AdminTestSuite))
}
