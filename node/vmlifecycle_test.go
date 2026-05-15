package node

import (
	"errors"
	"testing"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/vmm"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
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
	closed bool
}

func (vm *lifecycleVM) Apply(from string, meta vmmSchema.Meta) vmmSchema.Result {
	return vmmSchema.Result{}
}
func (vm *lifecycleVM) Checkpoint() (string, error) { return "vm-state", nil }
func (vm *lifecycleVM) Restore(data string) error   { return nil }
func (vm *lifecycleVM) Close() error {
	vm.closed = true
	return nil
}

func (suite *NodeVMLifecycleTestSuite) newLifecycleNode(pid string, vm vmmSchema.Vm, db *lifecycleDB) *Node {
	n := &Node{
		info: &nodeSchema.Info{Node: registrySchema.Node{AccId: "local-node"}},
		db:   db,
	}
	n.vmm = vmm.New(nil, n.info, nil, nil, nil)
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

func (suite *NodeVMLifecycleTestSuite) installLifecycleHooks(checkpointErr, saveErr, recoverErr error, registered bool) {
	oldCheckpoint := checkpointVM
	oldSave := saveCheckpoint
	oldRecover := recoverVM
	oldRegistered := processRegisteredToLocalNode
	suite.T().Cleanup(func() {
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

func (suite *NodeVMLifecycleTestSuite) TestStopVMRejectsCoreVM() {
	n := &Node{info: &nodeSchema.Info{Token: "token-pid", Registry: "registry-pid"}}
	n.vmm = vmm.New(nil, n.info, nil, nil, nil)

	err := n.StopVM("token-pid")

	assert.ErrorIs(suite.T(), err, nodeSchema.ErrCoreVmCannotStop)
}

func (suite *NodeVMLifecycleTestSuite) TestStopVMCheckpointFailureLeavesVMRunning() {
	pid := "pid-1"
	vm := &lifecycleVM{}
	n := suite.newLifecycleNode(pid, vm, &lifecycleDB{})
	suite.installLifecycleHooks(errors.New("checkpoint failed"), nil, nil, true)

	err := n.StopVM(pid)

	assert.Error(suite.T(), err)
	assert.True(suite.T(), n.vmm.IsExists(pid))
}

func (suite *NodeVMLifecycleTestSuite) TestStopVMSaveCheckpointIndexFailureLeavesVMRunning() {
	pid := "pid-1"
	vm := &lifecycleVM{}
	n := suite.newLifecycleNode(pid, vm, &lifecycleDB{saveCheckpointErr: errors.New("index failed")})
	suite.installLifecycleHooks(nil, nil, nil, true)

	err := n.StopVM(pid)

	assert.Error(suite.T(), err)
	assert.True(suite.T(), n.vmm.IsExists(pid))
}

func (suite *NodeVMLifecycleTestSuite) TestStopVMSuccessKillsVM() {
	pid := "pid-1"
	vm := &lifecycleVM{}
	db := &lifecycleDB{}
	n := suite.newLifecycleNode(pid, vm, db)
	suite.installLifecycleHooks(nil, nil, nil, true)

	err := n.StopVM(pid)

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), n.vmm.IsExists(pid))
	assert.True(suite.T(), vm.closed)
	assert.NotEmpty(suite.T(), db.saveCheckpointID)
}

func (suite *NodeVMLifecycleTestSuite) TestResumeVMSuccessRunsRecovery() {
	pid := "pid-1"
	vm := &lifecycleVM{}
	db := &lifecycleDB{nonce: 7, checkpointID: "ckp-1"}
	n := suite.newLifecycleNode(pid, vm, db)
	suite.installLifecycleHooks(nil, nil, nil, true)
	assert.NoError(suite.T(), n.vmm.Kill(pid))

	err := n.ResumeVM(pid)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), n.vmm.IsExists(pid))
}

func (suite *NodeVMLifecycleTestSuite) TestResumeVMAlreadyRunning() {
	pid := "pid-1"
	n := suite.newLifecycleNode(pid, &lifecycleVM{}, &lifecycleDB{})
	suite.installLifecycleHooks(nil, nil, nil, true)

	err := n.ResumeVM(pid)

	assert.ErrorIs(suite.T(), err, nodeSchema.ErrProcessAlreadyExists)
}

func (suite *NodeVMLifecycleTestSuite) TestResumeVMUnknownProcess() {
	pid := "pid-1"
	n := suite.newLifecycleNode(pid, &lifecycleVM{}, &lifecycleDB{})
	suite.installLifecycleHooks(nil, nil, nil, false)
	assert.NoError(suite.T(), n.vmm.Kill(pid))

	err := n.ResumeVM(pid)

	assert.ErrorIs(suite.T(), err, nodeSchema.ErrProcessNotFound)
}

func TestNodeVMLifecycleTestSuite(t *testing.T) {
	suite.Run(t, new(NodeVMLifecycleTestSuite))
}
