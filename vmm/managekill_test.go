package vmm

import (
	"errors"
	"sync"
	"testing"
	"time"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/vmm/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// VmmKillTestSuite tests VMM kill behavior.
type VmmKillTestSuite struct {
	suite.Suite
}

type killTestVM struct {
	closed       bool
	err          error
	closeEntered chan struct{}
	release      chan struct{}
}

func (v *killTestVM) Apply(from string, meta schema.Meta) schema.Result { return schema.Result{} }
func (v *killTestVM) Checkpoint() (string, error)                       { return "{}", nil }
func (v *killTestVM) Restore(data string) error                         { return nil }
func (v *killTestVM) Close() error {
	v.closed = true
	if v.closeEntered != nil {
		close(v.closeEntered)
		<-v.release
	}
	return v.err
}

func newKillTestVMM() *Vmm {
	return New(nil, &nodeSchema.Info{}, nil, nil, nil)
}

func (suite *VmmKillTestSuite) TestKillClosesAndRemovesVM() {
	v := newKillTestVMM()
	vm := &killTestVM{}
	v.addVm(vm, &schema.Env{Meta: schema.Meta{Pid: "pid-1"}})

	err := v.Kill("pid-1")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), vm.closed)
	assert.False(suite.T(), v.IsExists("pid-1"))
	assert.Empty(suite.T(), v.GetVmPids())
}

func (suite *VmmKillTestSuite) TestKillCloseFailureRemovesVMAndAllowsRestore() {
	v := newKillTestVMM()
	suite.T().Cleanup(v.Close)
	vm := &killTestVM{err: errors.New("close failed")}
	v.addVm(vm, &schema.Env{Meta: schema.Meta{Pid: "pid-1"}})

	err := v.Kill("pid-1")

	assert.ErrorIs(suite.T(), err, vm.err)
	assert.True(suite.T(), vm.closed)
	assert.False(suite.T(), v.IsExists("pid-1"))
	assert.Empty(suite.T(), v.GetVmPids())
	assert.Zero(suite.T(), v.GetVmCount())
	_, err = v.Checkpoint("pid-1")
	assert.ErrorIs(suite.T(), err, schema.ErrProcessNotFound)
	assert.ErrorIs(suite.T(), v.Kill("pid-1"), schema.ErrProcessNotFound)

	replacement := &killTestVM{}
	require.NoError(suite.T(), v.Mount("test.module", func(schema.Env) (schema.Vm, error) {
		return replacement, nil
	}))
	require.NoError(suite.T(), v.Restore(schema.Snapshot{Env: schema.Env{
		Meta:   schema.Meta{Pid: "pid-1"},
		Module: hymxSchema.Module{ModuleFormat: "test.module"},
	}}))
	assert.True(suite.T(), v.IsExists("pid-1"))
	_, err = v.Checkpoint("pid-1")
	assert.NoError(suite.T(), err)
	require.NoError(suite.T(), v.Kill("pid-1"))
	assert.True(suite.T(), replacement.closed)
}

func (suite *VmmKillTestSuite) TestCloseFailureRemovesVM() {
	v := newKillTestVMM()
	vm := &killTestVM{err: errors.New("close failed")}
	v.addVm(vm, &schema.Env{Meta: schema.Meta{Pid: "pid-1"}})

	v.Close()

	assert.True(suite.T(), vm.closed)
	assert.False(suite.T(), v.IsExists("pid-1"))
	assert.Empty(suite.T(), v.GetVmPids())
}

func (suite *VmmKillTestSuite) TestKillMissingProcessReturnsProcessNotFound() {
	v := newKillTestVMM()

	err := v.Kill("missing")

	assert.ErrorIs(suite.T(), err, schema.ErrProcessNotFound)
}

func (suite *VmmKillTestSuite) wait(done <-chan struct{}) {
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "timed out")
	}
}

func (suite *VmmKillTestSuite) TestKillAllStopsAllInstancesBeforeWaiting() {
	v := newKillTestVMM()
	suite.T().Cleanup(v.Close)
	a := &killTestVM{closeEntered: make(chan struct{}), release: make(chan struct{})}
	b := &killTestVM{closeEntered: make(chan struct{}), release: make(chan struct{}), err: errors.New("close failed")}
	v.addVm(a, &schema.Env{Meta: schema.Meta{Pid: "a"}})
	v.addVm(b, &schema.Env{Meta: schema.Meta{Pid: "b"}})
	var releaseA, releaseB sync.Once
	unblockA := func() { releaseA.Do(func() { close(a.release) }) }
	unblockB := func() { releaseB.Do(func() { close(b.release) }) }
	done := make(chan struct{})
	suite.T().Cleanup(func() {
		unblockA()
		unblockB()
		suite.wait(done)
	})
	go func() { v.KillAll(); close(done) }()

	// Both Close calls must start before either is released, regardless of map order.
	suite.wait(a.closeEntered)
	suite.wait(b.closeEntered)
	_, err := v.Checkpoint("a")
	assert.ErrorIs(suite.T(), err, schema.ErrVmStopping)
	_, err = v.Checkpoint("b")
	assert.ErrorIs(suite.T(), err, schema.ErrVmStopping)
	bInstance, err := v.getInstance("b")
	require.NoError(suite.T(), err)
	unblockB()
	suite.wait(bInstance.Done)
	assert.ErrorIs(suite.T(), bInstance.CloseErr, b.err)
	assert.False(suite.T(), v.IsExists("b"))
	assert.True(suite.T(), v.IsExists("a"))
	select {
	case <-done:
		require.FailNow(suite.T(), "KillAll returned before A closed")
	default:
	}

	unblockA()
	suite.wait(done)
	assert.Empty(suite.T(), v.GetVmPids())
	assert.Zero(suite.T(), v.GetVmCount())
}

func TestVmmKillTestSuite(t *testing.T) {
	suite.Run(t, new(VmmKillTestSuite))
}
