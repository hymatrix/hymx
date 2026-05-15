package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	serverSchema "github.com/hymatrix/hymx/server/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// AdminTestSuite tests admin HTTP handlers.
type AdminTestSuite struct {
	suite.Suite
}

func (suite *AdminTestSuite) TestAdminStopRouteReadsPidFromBodyAndUsesResponseEnvelope() {
	admin := &fakeVMAdmin{}
	s := &Server{vmAdmin: admin}
	engine := s.newAdminEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/vms/stop", bytes.NewBufferString(`{"pid":"pid-1"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "pid-1", admin.stoppedPid)
	res := serverSchema.Response{}
	assert.NoError(suite.T(), json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(suite.T(), "pid-1", res.Id)
	assert.Equal(suite.T(), "stopped", res.Message)
}

func (suite *AdminTestSuite) TestAdminResumeRouteReadsPidFromBodyAndUsesResponseEnvelope() {
	admin := &fakeVMAdmin{}
	s := &Server{vmAdmin: admin}
	engine := s.newAdminEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/vms/resume", bytes.NewBufferString(`{"pid":"pid-1"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "pid-1", admin.resumedPid)
	res := serverSchema.Response{}
	assert.NoError(suite.T(), json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(suite.T(), "pid-1", res.Id)
	assert.Equal(suite.T(), "resumed", res.Message)
}

func (suite *AdminTestSuite) TestAdminRunningRouteReturnsPids() {
	s := &Server{vmAdmin: &fakeVMAdmin{running: []string{"pid-1"}}}
	engine := s.newAdminEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/vms/running", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	res := serverSchema.ResponseRunningVMs{}
	assert.NoError(suite.T(), json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(suite.T(), []string{"pid-1"}, res.Pids)
}

func (suite *AdminTestSuite) TestAdminStopRouteReturnsErrorEnvelope() {
	s := &Server{vmAdmin: &fakeVMAdmin{err: errors.New("err_process_stopped")}}
	engine := s.newAdminEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/vms/stop", bytes.NewBufferString(`{"pid":"pid-1"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.JSONEq(suite.T(), `{"error":"err_process_stopped"}`, w.Body.String())
}

type fakeVMAdmin struct {
	running    []string
	stoppedPid string
	resumedPid string
	err        error
}

func (f *fakeVMAdmin) StopVM(pid string) error {
	f.stoppedPid = pid
	return f.err
}

func (f *fakeVMAdmin) ResumeVM(pid string) error {
	f.resumedPid = pid
	return f.err
}

func (f *fakeVMAdmin) GetRunningVMs() []string {
	return f.running
}

func TestAdminTestSuite(t *testing.T) {
	suite.Run(t, new(AdminTestSuite))
}
