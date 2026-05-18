package node

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hymatrix/hymx/db/cache"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/vmm"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// NodeVMLifecycleTestSuite tests Node VM lifecycle orchestration.
type NodeVMLifecycleTestSuite struct {
	suite.Suite
}

type lifecycleDB struct {
	nonce             int64
	checkpointID      string
	saveCheckpointID  string
	saveCheckpointErr error
	getNonceErr       error
	getCheckpointErr  error
}

func (db *lifecycleDB) SaveResult(result vmmSchema.VmmResult) error { return nil }
func (db *lifecycleDB) GetResult(msgid string) (*vmmSchema.VmmResult, error) {
	return nil, nil
}
func (db *lifecycleDB) GetResults(pid string, limit int64) ([]vmmSchema.VmmResult, error) {
	return nil, nil
}
func (db *lifecycleDB) IsExist(pid string) (bool, error) { return false, nil }
func (db *lifecycleDB) GetNonce(pid string) (int64, error) {
	return db.nonce, db.getNonceErr
}
func (db *lifecycleDB) Commit(pid string, nonce int64, msg, assign goarSchema.BundleItem) error {
	return nil
}
func (db *lifecycleDB) GetAllProcess() ([]string, []int64, error) { return nil, nil, nil }
func (db *lifecycleDB) GetMessage(msgid string) (*goarSchema.BundleItem, error) {
	return nil, nil
}
func (db *lifecycleDB) GetMessageByNonce(pid string, nonce int64) (*goarSchema.BundleItem, error) {
	return nil, nil
}
func (db *lifecycleDB) GetAssignByNonce(pid string, nonce int64) (*goarSchema.BundleItem, error) {
	return nil, nil
}
func (db *lifecycleDB) GetCheckpointIndex(pid string) (string, error) {
	return db.checkpointID, db.getCheckpointErr
}
func (db *lifecycleDB) SaveCheckpointIndex(pid, id string) error {
	db.saveCheckpointID = id
	return db.saveCheckpointErr
}
func (db *lifecycleDB) GetCache(pid, key string) (string, error) { return "", nil }
func (db *lifecycleDB) SaveCache(pid, key, value string) error   { return nil }

type lifecycleVM struct {
	checkpointErr error
	closed        bool
}

func (vm *lifecycleVM) Apply(from string, meta vmmSchema.Meta) vmmSchema.Result {
	return vmmSchema.Result{}
}
func (vm *lifecycleVM) Checkpoint() (string, error) {
	return "vm-state", vm.checkpointErr
}
func (vm *lifecycleVM) Restore(data string) error { return nil }
func (vm *lifecycleVM) Close() error {
	vm.closed = true
	return nil
}

func (suite *NodeVMLifecycleTestSuite) newLifecycleNode(pid string, vm vmmSchema.Vm, db *lifecycleDB) *Node {
	keyfile, err := filepath.Abs("../cmd/test_keyfile.json")
	assert.NoError(suite.T(), err)
	signer, err := goar.NewSignerFromPath(keyfile)
	assert.NoError(suite.T(), err)
	bundler, err := goar.NewBundler(signer)
	assert.NoError(suite.T(), err)

	oldWd, err := os.Getwd()
	assert.NoError(suite.T(), err)
	assert.NoError(suite.T(), os.Chdir(suite.T().TempDir()))
	suite.T().Cleanup(func() {
		assert.NoError(suite.T(), os.Chdir(oldWd))
	})

	n := &Node{
		info:     &nodeSchema.Info{Node: registrySchema.Node{AccId: "local-node"}},
		bundler:  bundler,
		db:       db,
		outboxDB: cache.NewOutbox(),
	}
	n.vmm = vmm.New(
		nil,
		n.info,
		make(chan vmmSchema.VmmResult, 100),
		make(chan vmmSchema.Outbox, 100),
		make(chan struct{}),
	)
	n.vmm.Run()
	suite.T().Cleanup(n.vmm.Close)
	assert.NoError(suite.T(), n.vmm.Mount("test.module", func(env vmmSchema.Env) (vmmSchema.Vm, error) {
		return vm, nil
	}))
	assert.NoError(suite.T(), n.vmm.Restore(vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta:   vmmSchema.Meta{Pid: pid},
			Module: hymxSchema.Module{ModuleFormat: "test.module"},
		},
		Data: "vm-state",
	}))
	return n
}

func (suite *NodeVMLifecycleTestSuite) registerProcess(n *Node, pid string) {
	accID := n.info.Node.AccId
	nodeJSON := fmt.Sprintf(`{"Acc-Id":%q,"Name":"local","Role":"main","Desc":"","URL":"http://127.0.0.1:8080"}`, accID)
	data := fmt.Sprintf(
		`{"i":"registry-pid","tp":"token-pid","mi":%q,"pi":{%q:{%q:%s}},"ai":{%q:{%q:%q}},"re":{%q:true},"n":{%q:%s}}`,
		accID,
		pid,
		accID,
		nodeJSON,
		accID,
		pid,
		pid,
		accID,
		accID,
		nodeJSON,
	)
	err := n.vmm.Restore(vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta: vmmSchema.Meta{
				Pid:   "registry-pid",
				AccId: "local-node",
				Params: map[string]string{
					"Token-Pid": "token-pid",
					"Name":      "local",
					"URL":       "http://127.0.0.1:8080",
				},
			},
			Process: hymxSchema.Process{Scheduler: "local-node"},
			Module:  hymxSchema.Module{ModuleFormat: vmmSchema.ModuleFormatRegistry},
		},
		Data: data,
	})
	assert.NoError(suite.T(), err)
}

func (suite *NodeVMLifecycleTestSuite) TestStopRejectsCoreProcess() {
	n := &Node{info: &nodeSchema.Info{Token: "token-pid", Registry: "registry-pid"}}
	n.vmm = vmm.New(nil, n.info, nil, nil, nil)

	err := n.Stop("token-pid")

	assert.ErrorIs(suite.T(), err, nodeSchema.ErrCoreProcessCannotStop)
}

func (suite *NodeVMLifecycleTestSuite) TestStopCheckpointFailureLeavesVMRunning() {
	pid := "pid-1"
	vm := &lifecycleVM{checkpointErr: errors.New("checkpoint failed")}
	n := suite.newLifecycleNode(pid, vm, &lifecycleDB{})
	suite.registerProcess(n, pid)

	err := n.Stop(pid)

	assert.Error(suite.T(), err)
	assert.True(suite.T(), n.vmm.IsExists(pid))
}

func (suite *NodeVMLifecycleTestSuite) TestStopSaveCheckpointIndexFailureLeavesVMRunning() {
	pid := "pid-1"
	vm := &lifecycleVM{}
	n := suite.newLifecycleNode(pid, vm, &lifecycleDB{saveCheckpointErr: errors.New("index failed")})
	suite.registerProcess(n, pid)

	err := n.Stop(pid)

	assert.Error(suite.T(), err)
	assert.True(suite.T(), n.vmm.IsExists(pid))
}

func (suite *NodeVMLifecycleTestSuite) TestSaveCheckpointPersistsItemAndIndex() {
	pid := "pid-1"
	db := &lifecycleDB{}
	n := suite.newLifecycleNode(pid, &lifecycleVM{}, db)

	ckpItem, err := n.saveCheckpoint(pid)

	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), ckpItem.Id)
	assert.Equal(suite.T(), ckpItem.Id, db.saveCheckpointID)
	savedItem, err := LoadCheckpoint(ckpItem.Id)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ckpItem.Id, savedItem.Id)
}

func (suite *NodeVMLifecycleTestSuite) TestStopSuccessKillsVM() {
	pid := "pid-1"
	vm := &lifecycleVM{}
	db := &lifecycleDB{}
	n := suite.newLifecycleNode(pid, vm, db)
	suite.registerProcess(n, pid)

	err := n.Stop(pid)

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), n.vmm.IsExists(pid))
	assert.True(suite.T(), vm.closed)
	assert.NotEmpty(suite.T(), db.saveCheckpointID)
}

func (suite *NodeVMLifecycleTestSuite) TestStopReturnsStoppedForRegisteredNonRunningProcess() {
	pid := "pid-1"
	n := suite.newLifecycleNode(pid, &lifecycleVM{}, &lifecycleDB{})
	suite.registerProcess(n, pid)
	assert.NoError(suite.T(), n.vmm.Kill(pid))

	err := n.Stop(pid)

	assert.ErrorIs(suite.T(), err, nodeSchema.ErrProcessStopped)
}

func (suite *NodeVMLifecycleTestSuite) TestStopReturnsNotFoundForUnknownNonRunningProcess() {
	pid := "pid-1"
	n := suite.newLifecycleNode(pid, &lifecycleVM{}, &lifecycleDB{})
	assert.NoError(suite.T(), n.vmm.Kill(pid))

	err := n.Stop(pid)

	assert.ErrorIs(suite.T(), err, nodeSchema.ErrProcessNotFound)
}

func (suite *NodeVMLifecycleTestSuite) TestResumeSuccessRunsRecovery() {
	pid := "pid-1"
	vm := &lifecycleVM{}
	db := &lifecycleDB{}
	n := suite.newLifecycleNode(pid, vm, db)
	suite.registerProcess(n, pid)
	err := n.Stop(pid)
	assert.NoError(suite.T(), err)
	db.checkpointID = db.saveCheckpointID

	err = n.Resume(pid)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), n.vmm.IsExists(pid))
}

func (suite *NodeVMLifecycleTestSuite) TestResumeAlreadyRunning() {
	pid := "pid-1"
	n := suite.newLifecycleNode(pid, &lifecycleVM{}, &lifecycleDB{})

	err := n.Resume(pid)

	assert.ErrorIs(suite.T(), err, nodeSchema.ErrProcessAlreadyExists)
}

func (suite *NodeVMLifecycleTestSuite) TestResumeUnknownProcess() {
	pid := "pid-1"
	n := suite.newLifecycleNode(pid, &lifecycleVM{}, &lifecycleDB{})
	assert.NoError(suite.T(), n.vmm.Kill(pid))

	err := n.Resume(pid)

	assert.ErrorIs(suite.T(), err, nodeSchema.ErrProcessNotFound)
}

func (suite *NodeVMLifecycleTestSuite) TestHandleMessageReturnsStoppedErrorForRegisteredNonRunningProcess() {
	pid := "pid-1"
	n := suite.newLifecycleNode(pid, &lifecycleVM{}, &lifecycleDB{})
	n.info.Node.AccId = n.bundler.Address
	suite.registerProcess(n, pid)
	assert.NoError(suite.T(), n.vmm.Kill(pid))

	err := n.handleMessage(pid, "accid", goarSchema.BundleItem{}, hymxSchema.Message{})

	assert.ErrorIs(suite.T(), err, nodeSchema.ErrProcessStopped)
}

func TestNodeVMLifecycleTestSuite(t *testing.T) {
	suite.Run(t, new(NodeVMLifecycleTestSuite))
}
