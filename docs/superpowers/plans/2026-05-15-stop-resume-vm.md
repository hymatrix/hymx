# Stop/Resume VM Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add admin-only APIs to stop and resume VMs while keeping VMM stateless about stopped processes.

**Architecture:** VMM remains a running-VM container and exposes running pid data through existing `GetVmPids()`. `Node` orchestrates checkpoint, `vmm.Kill`, rollback, Registry-based stopped detection, and one-process recovery. `Server` exposes a separate admin server only when `adminPort` is configured; stop/resume read `pid` from JSON bodies and return the existing `server/schema.Response` envelope.

**Tech Stack:** Go, Gin, existing Hymx `node`, `vmm`, `server`, Registry APIs, checkpoint/recovery code, `go test`.

---

## File Structure

- Modify `vmm/manage.go`: make `Kill` return `vm.Close()` errors without deleting the VM, and keep `GetVmPids()` as the running VM list source.
- Add `vmm/manage_kill_test.go`: unit tests for `Kill` success and close failure.
- Modify `node/schema/errors.go`: add `err_process_stopped` and `err_core_vm_cannot_stop`.
- Add `node/vm_lifecycle.go`: implement `StopVM`, `ResumeVM`, `GetRunningVMs`, rollback, and Registry-local-process helper.
- Modify `node/message.go`: derive stopped state when a pid is registered locally but absent from VMM.
- Add `node/vm_lifecycle_test.go`: unit tests for lifecycle orchestration and stopped-message classification using fake DB/VM hooks.
- Modify `server/server.go`: add admin server field, `Run` signature, admin close path, and VM admin interface.
- Add `server/admin.go`: admin Gin engine, body parsing, routes, handlers, and close helper.
- Modify `server/schema/schema.go`: add `VMRequest` and `RunningVMsResponse`.
- Add `server/admin_test.go`: handler tests for body pid and response envelope.
- Modify `cmd/cfgnode.go`: read `adminPort`.
- Modify `cmd/main.go`: pass `adminPort` into `Server.Run`.
- Modify checked-in `cmd/config*.yaml`: add disabled `adminPort` examples.
- Modify `docs/api.md`: document admin API, body params, running VM list, and stopped-message behavior.

## Task 1: Tighten VMM Kill Semantics

**Files:**
- Modify: `vmm/manage.go`
- Test: `vmm/manage_kill_test.go`

- [ ] **Step 1: Write failing VMM Kill tests**

Create `vmm/manage_kill_test.go`:

```go
package vmm

import (
	"errors"
	"testing"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/vmm/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type killTestVM struct {
	closed bool
	err    error
}

func (v *killTestVM) Apply(from string, meta schema.Meta) schema.Result { return schema.Result{} }
func (v *killTestVM) Checkpoint() (string, error)                       { return "{}", nil }
func (v *killTestVM) Restore(data string) error                         { return nil }
func (v *killTestVM) Close() error {
	v.closed = true
	return v.err
}

func newKillTestVMM() *Vmm {
	return New(nil, &nodeSchema.Info{}, nil, nil, nil)
}

func TestKillClosesAndRemovesVM(t *testing.T) {
	v := newKillTestVMM()
	vm := &killTestVM{}
	v.addVm(vm, &schema.Env{Meta: schema.Meta{Pid: "pid-1"}})

	err := v.Kill("pid-1")

	require.NoError(t, err)
	assert.True(t, vm.closed)
	assert.False(t, v.IsExists("pid-1"))
	assert.Empty(t, v.GetVmPids())
}

func TestKillCloseFailureKeepsVM(t *testing.T) {
	v := newKillTestVMM()
	vm := &killTestVM{err: errors.New("close failed")}
	v.addVm(vm, &schema.Env{Meta: schema.Meta{Pid: "pid-1"}})

	err := v.Kill("pid-1")

	require.Error(t, err)
	assert.True(t, vm.closed)
	assert.True(t, v.IsExists("pid-1"))
	assert.Equal(t, []string{"pid-1"}, v.GetVmPids())
}

func TestKillMissingProcessReturnsProcessNotFound(t *testing.T) {
	v := newKillTestVMM()

	err := v.Kill("missing")

	assert.ErrorIs(t, err, schema.ErrProcessNotFound)
}
```

- [ ] **Step 2: Run VMM Kill tests and verify failure**

Run:

```bash
go test ./vmm -run TestKill -count=1
```

Expected: FAIL because current `Kill` deletes the VM even when `Close` fails.

- [ ] **Step 3: Update Kill implementation**

Modify `vmm/manage.go`:

```go
func (v *Vmm) Kill(pid string) (err error) {
	vm, _, err := v.GetVm(pid)
	if err != nil {
		return
	}

	v.vmsLockMu.Lock()
	defer v.vmsLockMu.Unlock()
	if err = vm.Close(); err != nil {
		return err
	}
	delete(v.vms, pid)
	delete(v.vmsEnv, pid)

	return
}
```

- [ ] **Step 4: Run VMM tests**

Run:

```bash
go test ./vmm -run TestKill -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit VMM Kill behavior**

Run:

```bash
git add vmm/manage.go vmm/manage_kill_test.go
git commit -m "fix: keep vm when kill close fails"
```

## Task 2: Add Node VM Lifecycle Orchestration

**Files:**
- Modify: `node/schema/errors.go`
- Add: `node/vm_lifecycle.go`
- Test: `node/vm_lifecycle_test.go`

- [ ] **Step 1: Add Node lifecycle errors**

Modify `node/schema/errors.go`:

```go
ErrProcessStopped   = errors.New("err_process_stopped")
ErrCoreVmCannotStop = errors.New("err_core_vm_cannot_stop")
```

- [ ] **Step 2: Write failing lifecycle tests**

Create `node/vm_lifecycle_test.go`:

```go
package node

import (
	"errors"
	"testing"

	hymxSchema "github.com/hymatrix/hymx/schema"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/vmm"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lifecycleDB struct {
	nonce             int64
	checkpointID      string
	saveCheckpointID  string
	saveCheckpointErr error
	getNonceErr       error
	getCheckpointErr  error
}

func (db *lifecycleDB) SaveResult(result vmmSchema.VmmResult) error { return nil }
func (db *lifecycleDB) GetResult(msgid string) (*vmmSchema.VmmResult, error) { return nil, nil }
func (db *lifecycleDB) GetResults(pid string, limit int64) ([]vmmSchema.VmmResult, error) { return nil, nil }
func (db *lifecycleDB) IsExist(pid string) (bool, error) { return false, nil }
func (db *lifecycleDB) GetNonce(pid string) (int64, error) { return db.nonce, db.getNonceErr }
func (db *lifecycleDB) Commit(pid string, nonce int64, msg, assign goarSchema.BundleItem) error { return nil }
func (db *lifecycleDB) GetAllProcess() ([]string, []int64, error) { return nil, nil, nil }
func (db *lifecycleDB) GetMessage(msgid string) (*goarSchema.BundleItem, error) { return nil, nil }
func (db *lifecycleDB) GetMessageByNonce(pid string, nonce int64) (*goarSchema.BundleItem, error) { return nil, nil }
func (db *lifecycleDB) GetAssignByNonce(pid string, nonce int64) (*goarSchema.BundleItem, error) { return nil, nil }
func (db *lifecycleDB) GetCheckpointIndex(pid string) (string, error) { return db.checkpointID, db.getCheckpointErr }
func (db *lifecycleDB) SaveCheckpointIndex(pid, id string) error {
	db.saveCheckpointID = id
	return db.saveCheckpointErr
}
func (db *lifecycleDB) GetCache(pid, key string) (string, error) { return "", nil }
func (db *lifecycleDB) SaveCache(pid, key, value string) error { return nil }

type lifecycleVM struct {
	closed bool
}

func (vm *lifecycleVM) Apply(from string, meta vmmSchema.Meta) vmmSchema.Result { return vmmSchema.Result{} }
func (vm *lifecycleVM) Checkpoint() (string, error) { return "vm-state", nil }
func (vm *lifecycleVM) Restore(data string) error { return nil }
func (vm *lifecycleVM) Close() error {
	vm.closed = true
	return nil
}

func newLifecycleNode(t *testing.T, pid string, vm vmmSchema.Vm, db *lifecycleDB) *Node {
	t.Helper()
	n := &Node{
		info: &nodeSchema.Info{Node: registrySchema.Node{AccId: "local-node"}},
		db:   db,
	}
	n.vmm = vmm.New(nil, n.info, nil, nil, nil)
	require.NoError(t, n.vmm.Mount("test.module", func(env vmmSchema.Env) (vmmSchema.Vm, error) {
		return vm, nil
	}))
	require.NoError(t, n.vmm.Restore(vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta:   vmmSchema.Meta{Pid: pid},
			Module: hymxSchema.Module{ModuleFormat: "test.module"},
		},
		Data: "vm-state",
	}))
	return n
}

func installLifecycleHooks(t *testing.T, checkpointErr, saveErr, recoverErr error, registered bool) {
	t.Helper()
	oldCheckpoint := checkpointVM
	oldSave := saveCheckpoint
	oldRecover := recoverVM
	oldRegistered := processRegisteredToLocalNode
	t.Cleanup(func() {
		checkpointVM = oldCheckpoint
		saveCheckpoint = oldSave
		recoverVM = oldRecover
		processRegisteredToLocalNode = oldRegistered
	})

	checkpointVM = func(n *Node, pid string) (goarSchema.BundleItem, error) {
		return goarSchema.BundleItem{Id: "ckp-1"}, checkpointErr
	}
	saveCheckpoint = func(goarSchema.BundleItem) error {
		return saveErr
	}
	recoverVM = func(n *Node, pid string, maxNonce int64, ckpId string, mode vmmSchema.ExecMode) error {
		if recoverErr != nil {
			return recoverErr
		}
		return n.vmm.Restore(vmmSchema.Snapshot{
			Env: vmmSchema.Env{
				Meta:   vmmSchema.Meta{Pid: pid},
				Module: hymxSchema.Module{ModuleFormat: "test.module"},
			},
			Data: "vm-state",
		})
	}
	processRegisteredToLocalNode = func(n *Node, pid string) (bool, error) {
		return registered, nil
	}
}

func TestStopVMRejectsCoreVM(t *testing.T) {
	n := &Node{info: &nodeSchema.Info{Token: "token-pid", Registry: "registry-pid"}}
	n.vmm = vmm.New(nil, n.info, nil, nil, nil)

	err := n.StopVM("token-pid")

	assert.ErrorIs(t, err, nodeSchema.ErrCoreVmCannotStop)
}

func TestStopVMCheckpointFailureLeavesVMRunning(t *testing.T) {
	pid := "pid-1"
	vm := &lifecycleVM{}
	n := newLifecycleNode(t, pid, vm, &lifecycleDB{})
	installLifecycleHooks(t, errors.New("checkpoint failed"), nil, nil, true)

	err := n.StopVM(pid)

	require.Error(t, err)
	assert.True(t, n.vmm.IsExists(pid))
}

func TestStopVMSaveCheckpointIndexFailureLeavesVMRunning(t *testing.T) {
	pid := "pid-1"
	vm := &lifecycleVM{}
	n := newLifecycleNode(t, pid, vm, &lifecycleDB{saveCheckpointErr: errors.New("index failed")})
	installLifecycleHooks(t, nil, nil, nil, true)

	err := n.StopVM(pid)

	require.Error(t, err)
	assert.True(t, n.vmm.IsExists(pid))
}

func TestStopVMSuccessKillsVM(t *testing.T) {
	pid := "pid-1"
	vm := &lifecycleVM{}
	db := &lifecycleDB{}
	n := newLifecycleNode(t, pid, vm, db)
	installLifecycleHooks(t, nil, nil, nil, true)

	err := n.StopVM(pid)

	require.NoError(t, err)
	assert.False(t, n.vmm.IsExists(pid))
	assert.True(t, vm.closed)
	assert.NotEmpty(t, db.saveCheckpointID)
}

func TestResumeVMSuccessRunsRecovery(t *testing.T) {
	pid := "pid-1"
	vm := &lifecycleVM{}
	db := &lifecycleDB{nonce: 7, checkpointID: "ckp-1"}
	n := newLifecycleNode(t, pid, vm, db)
	installLifecycleHooks(t, nil, nil, nil, true)
	require.NoError(t, n.vmm.Kill(pid))

	err := n.ResumeVM(pid)

	require.NoError(t, err)
	assert.True(t, n.vmm.IsExists(pid))
}

func TestResumeVMAlreadyRunning(t *testing.T) {
	pid := "pid-1"
	n := newLifecycleNode(t, pid, &lifecycleVM{}, &lifecycleDB{})
	installLifecycleHooks(t, nil, nil, nil, true)

	err := n.ResumeVM(pid)

	assert.ErrorIs(t, err, nodeSchema.ErrProcessAlreadyExists)
}

func TestResumeVMUnknownProcess(t *testing.T) {
	pid := "pid-1"
	n := newLifecycleNode(t, pid, &lifecycleVM{}, &lifecycleDB{})
	installLifecycleHooks(t, nil, nil, nil, false)
	require.NoError(t, n.vmm.Kill(pid))

	err := n.ResumeVM(pid)

	assert.ErrorIs(t, err, nodeSchema.ErrProcessNotFound)
}
```

- [ ] **Step 3: Run lifecycle tests and verify failure**

Run:

```bash
go test ./node -run 'TestStopVM|TestResumeVM' -count=1
```

Expected: FAIL because `StopVM`, `ResumeVM`, and lifecycle hooks do not exist.

- [ ] **Step 4: Implement Node lifecycle file**

Create `node/vm_lifecycle.go`:

```go
package node

import (
	"github.com/hymatrix/hymx/node/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

var checkpointVM = func(n *Node, pid string) (goarSchema.BundleItem, error) {
	return n.Checkpoint(pid)
}

var saveCheckpoint = SaveCheckpoint

var recoverVM = func(n *Node, pid string, maxNonce int64, ckpId string, mode vmmSchema.ExecMode) error {
	return n.recoveryProcess(pid, maxNonce, ckpId, mode)
}

var processRegisteredToLocalNode = func(n *Node, pid string) (bool, error) {
	nodes, err := n.GetNodesByProcess(pid)
	if err != nil {
		return false, err
	}
	if n.info == nil {
		return false, nil
	}
	for _, node := range nodes {
		if node.AccId == n.info.Node.AccId {
			return true, nil
		}
	}
	return false, nil
}

func (n *Node) StopVM(pid string) error {
	if n.isCoreVM(pid) {
		return schema.ErrCoreVmCannotStop
	}
	if n.vmm.IsRecovering(pid) {
		return schema.ErrProcessIsRecovering
	}
	if !n.vmm.IsExists(pid) {
		return schema.ErrProcessNotFound
	}

	ckpItem, err := checkpointVM(n, pid)
	if err != nil {
		return err
	}
	if err = saveCheckpoint(ckpItem); err != nil {
		return err
	}
	if err = n.db.SaveCheckpointIndex(pid, ckpItem.Id); err != nil {
		return err
	}

	if err = n.vmm.Kill(pid); err != nil {
		n.restoreAfterFailedStop(pid, ckpItem.Id, err)
		return err
	}
	return nil
}

func (n *Node) ResumeVM(pid string) error {
	if n.vmm.IsExists(pid) {
		return schema.ErrProcessAlreadyExists
	}

	registered, err := processRegisteredToLocalNode(n, pid)
	if err != nil {
		return err
	}
	if !registered {
		return schema.ErrProcessNotFound
	}

	maxNonce, err := n.db.GetNonce(pid)
	if err != nil {
		return err
	}
	ckpId, err := n.db.GetCheckpointIndex(pid)
	if err != nil {
		ckpId = ""
	}

	return recoverVM(n, pid, maxNonce, ckpId, vmmSchema.ExecModeDryRun)
}

func (n *Node) GetRunningVMs() []string {
	return n.vmm.GetVmPids()
}

func (n *Node) isCoreVM(pid string) bool {
	if pid == "" {
		return false
	}
	if n.vmm != nil && (pid == n.vmm.TokenId() || pid == n.vmm.RegistryId()) {
		return true
	}
	if n.info != nil && (pid == n.info.Token || pid == n.info.Registry) {
		return true
	}
	return false
}

func (n *Node) restoreAfterFailedStop(pid, ckpId string, originalErr error) {
	maxNonce, err := n.db.GetNonce(pid)
	if err != nil {
		log.Error("failed to get nonce during stop rollback", "pid", pid, "originalErr", originalErr, "err", err)
		return
	}
	if err = recoverVM(n, pid, maxNonce, ckpId, vmmSchema.ExecModeDryRun); err != nil {
		log.Error("failed to recover vm during stop rollback", "pid", pid, "originalErr", originalErr, "err", err)
	}
}
```

- [ ] **Step 5: Run lifecycle tests**

Run:

```bash
go test ./node -run 'TestStopVM|TestResumeVM' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Node lifecycle orchestration**

Run:

```bash
git add node/schema/errors.go node/vm_lifecycle.go node/vm_lifecycle_test.go
git commit -m "feat: add node vm lifecycle orchestration"
```

## Task 3: Derive Stopped Message Error

**Files:**
- Modify: `node/message.go`
- Test: `node/vm_lifecycle_test.go`

- [ ] **Step 1: Write stopped classification tests**

Add to `node/vm_lifecycle_test.go`:

```go
func TestStoppedErrorForRegisteredNonRunningProcess(t *testing.T) {
	pid := "pid-1"
	n := newLifecycleNode(t, pid, &lifecycleVM{}, &lifecycleDB{})
	installLifecycleHooks(t, nil, nil, nil, true)
	require.NoError(t, n.vmm.Kill(pid))

	err := n.errForMissingLocalVM(pid)

	assert.ErrorIs(t, err, nodeSchema.ErrProcessStopped)
}

func TestProcessNotFoundForUnknownNonRunningProcess(t *testing.T) {
	pid := "pid-1"
	n := newLifecycleNode(t, pid, &lifecycleVM{}, &lifecycleDB{})
	installLifecycleHooks(t, nil, nil, nil, false)
	require.NoError(t, n.vmm.Kill(pid))

	err := n.errForMissingLocalVM(pid)

	assert.ErrorIs(t, err, nodeSchema.ErrProcessNotFound)
}
```

- [ ] **Step 2: Run classification tests and verify failure**

Run:

```bash
go test ./node -run 'TestStoppedError|TestProcessNotFoundForUnknown' -count=1
```

Expected: FAIL because `errForMissingLocalVM` does not exist.

- [ ] **Step 3: Add missing-VM classifier and use it**

Modify `node/message.go`:

```go
func (n *Node) errForMissingLocalVM(pid string) error {
	registered, err := processRegisteredToLocalNode(n, pid)
	if err != nil {
		return err
	}
	if registered {
		return schema.ErrProcessStopped
	}
	return schema.ErrProcessNotFound
}
```

Replace the existing not-found block in `handleMessage`:

```go
if !n.vmm.IsExists(pid) {
	err = n.errForMissingLocalVM(pid)
	log.Error("handle message failed", "pid", pid, "err", err)
	return
}
```

Leave the existing redirect check before this block.

- [ ] **Step 4: Run focused Node tests**

Run:

```bash
go test ./node -run 'TestStoppedError|TestProcessNotFoundForUnknown|TestStopVM|TestResumeVM' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit stopped message classification**

Run:

```bash
git add node/message.go node/vm_lifecycle_test.go
git commit -m "feat: return stopped error for registered inactive vm"
```

## Task 4: Add Admin HTTP Server

**Files:**
- Modify: `server/server.go`
- Add: `server/admin.go`
- Modify: `server/schema/schema.go`
- Test: `server/admin_test.go`

- [ ] **Step 1: Write failing admin handler tests**

Create `server/admin_test.go`:

```go
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
	"github.com/stretchr/testify/require"
)

func TestAdminStopRouteReadsPidFromBodyAndUsesResponseEnvelope(t *testing.T) {
	admin := &fakeVMAdmin{}
	s := &Server{vmAdmin: admin}
	engine := s.newAdminEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/vms/stop", bytes.NewBufferString(`{"pid":"pid-1"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pid-1", admin.stoppedPid)
	var res serverSchema.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, "pid-1", res.Id)
	assert.Equal(t, "stopped", res.Message)
}

func TestAdminResumeRouteReadsPidFromBodyAndUsesResponseEnvelope(t *testing.T) {
	admin := &fakeVMAdmin{}
	s := &Server{vmAdmin: admin}
	engine := s.newAdminEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/vms/resume", bytes.NewBufferString(`{"pid":"pid-1"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pid-1", admin.resumedPid)
	var res serverSchema.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, "pid-1", res.Id)
	assert.Equal(t, "resumed", res.Message)
}

func TestAdminRunningRouteReturnsPids(t *testing.T) {
	s := &Server{vmAdmin: &fakeVMAdmin{running: []string{"pid-1"}}}
	engine := s.newAdminEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/vms/running", nil)
	engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var res serverSchema.RunningVMsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, []string{"pid-1"}, res.Pids)
}

func TestAdminStopRouteReturnsErrorEnvelope(t *testing.T) {
	s := &Server{vmAdmin: &fakeVMAdmin{err: errors.New("err_process_stopped")}}
	engine := s.newAdminEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/vms/stop", bytes.NewBufferString(`{"pid":"pid-1"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"err_process_stopped"}`, w.Body.String())
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
```

- [ ] **Step 2: Run admin tests and verify failure**

Run:

```bash
go test ./server -run TestAdmin -count=1
```

Expected: FAIL because `vmAdmin`, `newAdminEngine`, `VMRequest`, and `RunningVMsResponse` do not exist.

- [ ] **Step 3: Add admin request and running response structs**

Modify `server/schema/schema.go`:

```go
type VMRequest struct {
	Pid string `json:"pid"`
}

type RunningVMsResponse struct {
	Pids []string `json:"pids"`
}
```

- [ ] **Step 4: Add admin interface and server field**

Modify `server/server.go`:

```go
type vmAdmin interface {
	StopVM(pid string) error
	ResumeVM(pid string) error
	GetRunningVMs() []string
}

type Server struct {
	node *node.Node
	pay  *pay.Pay

	vmAdmin vmAdmin

	apiServer      *http.Server
	adminAPIServer *http.Server
}
```

Modify `New`:

```go
return &Server{
	node:    node,
	pay:     pay,
	vmAdmin: node,
}
```

- [ ] **Step 5: Implement admin routes**

Create `server/admin.go`:

```go
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/server/schema"
)

func (s *Server) runAdminAPI(endpoint string) {
	if endpoint == "" {
		return
	}

	engine := s.newAdminEngine()
	s.adminAPIServer = &http.Server{
		Addr:    endpoint,
		Handler: engine,
	}

	if err := s.adminAPIServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("admin http ListenAndServe", "err", err)
	}
}

func (s *Server) newAdminEngine() *gin.Engine {
	engine := gin.Default()
	engine.Use(common.CORSMiddleware())
	engine.POST("/admin/vms/stop", s.AdminStopVM)
	engine.POST("/admin/vms/resume", s.AdminResumeVM)
	engine.GET("/admin/vms/running", s.AdminRunningVMs)
	return engine
}

func (s *Server) closeAdminAPI() {
	if s.adminAPIServer == nil {
		return
	}

	log.Info("admin api is shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := s.adminAPIServer.Shutdown(ctx); err != nil {
		log.Error("failed to shut down the admin api", "err", err)
		s.adminAPIServer.Close()
		return
	}
	log.Info("admin api has been shut down")
}

func (s *Server) AdminStopVM(c *gin.Context) {
	var req schema.VMRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Pid == "" {
		schema.ErrorResponse(c, schema.ErrInvalidParams.Error())
		return
	}
	if err := s.vmAdmin.StopVM(req.Pid); err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, schema.Response{Id: req.Pid, Message: "stopped"})
}

func (s *Server) AdminResumeVM(c *gin.Context) {
	var req schema.VMRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Pid == "" {
		schema.ErrorResponse(c, schema.ErrInvalidParams.Error())
		return
	}
	if err := s.vmAdmin.ResumeVM(req.Pid); err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, schema.Response{Id: req.Pid, Message: "resumed"})
}

func (s *Server) AdminRunningVMs(c *gin.Context) {
	c.JSON(http.StatusOK, schema.RunningVMsResponse{Pids: s.vmAdmin.GetRunningVMs()})
}
```

- [ ] **Step 6: Wire admin server lifecycle**

Modify `server/server.go`:

```go
func (s *Server) Run(endpoint string, adminEndpoint string, startMode string) {
	if s.pay != nil {
		s.pay.LoadCheckpoint()
		s.pay.Run()
		s.AddResultHandler(s.pay.HymxDepositHandler)
		s.AddItemHandler(s.pay.HymxFeeHandler)
	}

	go s.runAPI(endpoint)
	if adminEndpoint != "" {
		go s.runAdminAPI(adminEndpoint)
	}

	s.node.Run(startMode)
}
```

Modify `Close`:

```go
s.closeAPI()
s.closeAdminAPI()
s.node.Close()
```

- [ ] **Step 7: Run server admin tests**

Run:

```bash
go test ./server -run TestAdmin -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit admin server**

Run:

```bash
git add server/server.go server/admin.go server/schema/schema.go server/admin_test.go
git commit -m "feat: add admin vm lifecycle api"
```

## Task 5: Wire Config and CLI Startup

**Files:**
- Modify: `cmd/cfgnode.go`
- Modify: `cmd/main.go`
- Modify: `cmd/config.yaml`
- Modify: `cmd/config_test_network.yaml`
- Modify: `cmd/config2.yaml`
- Modify: `cmd/config_payment.yaml`
- Modify: `cmd/config_chainkit.yaml`

- [ ] **Step 1: Update config loader signature**

Modify `cmd/cfgnode.go` signature:

```go
func LoadNodeConfig() (
	port, adminPort, ginMode, redisURL, arweaveURL, hymxURL string,
	bundler *goar.Bundler, nodeInfo *nodeSchema.Info, decryptor *cryptor.Cryptor, err error,
) {
	port = viper.GetString("port")
	adminPort = viper.GetString("adminPort")
	ginMode = viper.GetString("ginMode")
```

- [ ] **Step 2: Pass adminPort to Server.Run**

Modify `cmd/main.go`:

```go
port, adminPort, ginMode, redisURL, arweaveURL, hymxURL, bundler, nodeInfo, decryptor, err := LoadNodeConfig()
```

Modify the run call:

```go
s.Run(port, adminPort, c.String("mode"))
```

Update the log context:

```go
log.Info("server is running", "protocol version", schema.Variant, "node version", nodeSchema.NodeVersion, "wallet", bundler.Address, "port", port, "adminPort", adminPort)
```

- [ ] **Step 3: Add disabled adminPort examples**

Add this line near `port` in each checked-in `cmd/config*.yaml`:

```yaml
adminPort: "" # optional, e.g. :8081; empty disables admin API
```

- [ ] **Step 4: Run command and focused package tests**

Run:

```bash
go test ./cmd -count=1
go test ./server ./node ./vmm -count=1
```

Expected: PASS or no test files for `./cmd`; PASS for focused packages.

- [ ] **Step 5: Commit config wiring**

Run:

```bash
git add cmd/cfgnode.go cmd/main.go cmd/config.yaml cmd/config_test_network.yaml cmd/config2.yaml cmd/config_payment.yaml cmd/config_chainkit.yaml
git commit -m "feat: wire admin port config"
```

## Task 6: Document API Behavior

**Files:**
- Modify: `docs/api.md`

- [ ] **Step 1: Update API docs**

Add this section to `docs/api.md` after the Utilities section:

```markdown
### Admin VM Lifecycle

Admin VM lifecycle endpoints are served only on the configured `adminPort`. If `adminPort` is empty or missing, the admin server is not started and these endpoints are unavailable.

- `POST /admin/vms/stop`
  - Description: Checkpoint and stop a live VM process on this node. The process remains registered in Registry.
  - Request: `{ "pid": "<process-id>" }`
  - Success: `200` with `{ "id": "<pid>", "message": "stopped" }`
  - Errors:
    - `400` with `{ "error": "err_invalid_params" }` when `pid` is missing
    - `400` with `{ "error": "err_core_vm_cannot_stop" }` for token or registry
    - `400` with `{ "error": "err_process_not_found" }` when the VM is not live

- `POST /admin/vms/resume`
  - Description: Resume a registered but non-running VM by running recovery for that process.
  - Request: `{ "pid": "<process-id>" }`
  - Success: `200` with `{ "id": "<pid>", "message": "resumed" }`
  - Errors:
    - `400` with `{ "error": "err_invalid_params" }` when `pid` is missing
    - `400` with `{ "error": "err_process_already_exist" }` when the VM is already running
    - `400` with `{ "error": "err_process_not_found" }` when the process is not registered to this node

- `GET /admin/vms/running`
  - Description: List VM pids currently running in this node process.
  - Success: `200` with `{ "pids": ["<pid>"] }`

Clients can derive stopped VMs by comparing `GET /processes/:accid` with `GET /admin/vms/running`.
```

Add this bullet to `POST /` errors:

```markdown
- `400` with `{ "error": "err_process_stopped" }` when the target process is registered to this node but not currently running. The message is not assigned or persisted.
```

- [ ] **Step 2: Run docs diff check**

Run:

```bash
git diff -- docs/api.md
```

Expected: diff documents body-based admin API, running VM list, and stopped-message behavior.

- [ ] **Step 3: Commit docs**

Run:

```bash
git add docs/api.md
git commit -m "docs: document vm lifecycle admin api"
```

## Task 7: Full Verification

**Files:**
- Verify all changed files.

- [ ] **Step 1: Run focused tests**

Run:

```bash
go test ./vmm ./node ./server ./cmd -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full test suite**

Run:

```bash
go test ./... -count=1
```

Expected: PASS. If Redis-dependent tests fail because local Redis is not running, record the exact failing packages and rerun the focused non-Redis packages:

```bash
go test ./vmm ./node ./server ./cmd ./sdk ./cryptor ./db/cache -count=1
```

- [ ] **Step 3: Inspect final git state**

Run:

```bash
git status --short
git log --oneline -5
```

Expected: only intentional files are changed or committed. The pre-existing untracked `sdk/mod/` may still appear and must not be staged unless the user explicitly asks.

- [ ] **Step 4: Manual smoke test with admin disabled**

Run the node with a config where `adminPort: ""`:

```bash
go run ./cmd --config ./cmd/config.yaml --mode normal
```

Expected: public server starts on `port`; no admin server starts. Stop the process with Ctrl-C after confirming startup logs.

- [ ] **Step 5: Manual smoke test with admin enabled**

Temporarily set `adminPort: ":8081"` in a local config copy, then run:

```bash
go run ./cmd --config ./cmd/config.yaml --mode normal
```

In another terminal:

```bash
curl http://127.0.0.1:8081/admin/vms/running
```

Expected:

```json
{"pids":[]}
```

Restore the config value to `adminPort: ""` before committing if the checked-in config should remain disabled.

## Self-Review

- Spec coverage: body-based admin stop/resume, shared response envelope, no VMM stopped state, running VM list, `Kill` reuse, checkpoint-before-stop, rollback, resume through recovery, Registry-derived stopped message error, core VM protection, and no Registry mutation are covered by tasks.
- Placeholder scan: this plan contains no placeholders.
- Type consistency: VMM uses existing `Kill` and `GetVmPids`; Node uses `StopVM`, `ResumeVM`, `GetRunningVMs`; server uses `VMRequest`, `RunningVMsResponse`, and existing `Response`.
