package vmm

import (
	"sync"
	"testing"
	"time"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hySchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/vmm/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type VmmQueueTestSuite struct{ suite.Suite }

type queueTestVM struct {
	entered chan struct{}
	release <-chan struct{}
	nonces  []int64
	closed  chan struct{}
}

func (vm *queueTestVM) Apply(_ string, meta schema.Meta) schema.Result {
	if meta.Nonce == 1 && vm.entered != nil {
		close(vm.entered)
		<-vm.release
	}
	vm.nonces = append(vm.nonces, meta.Nonce)
	return schema.Result{}
}
func (vm *queueTestVM) Checkpoint() (string, error) { return "snapshot", nil }
func (vm *queueTestVM) Restore(string) error        { return nil }
func (vm *queueTestVM) Close() error {
	if vm.closed != nil {
		close(vm.closed)
	}
	return nil
}

func (suite *VmmQueueTestSuite) wait(done <-chan struct{}) {
	suite.T().Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "operation blocked")
	}
}

func (suite *VmmQueueTestSuite) TestApplyAcceptsBeyondOldCapacityAndPreservesFIFO() {
	const count = 2048
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	v := New(nil, &nodeSchema.Info{}, make(chan schema.VmmResult, count+4), make(chan schema.Outbox, count+4), nil)
	vm := &queueTestVM{entered: make(chan struct{}), release: release}
	v.addVm(vm, &schema.Env{Meta: schema.Meta{Pid: "a"}})
	v.addVm(&queueTestVM{}, &schema.Env{Meta: schema.Meta{Pid: "b"}})
	suite.T().Cleanup(func() { unblock(); v.Close() })
	v.Apply(schema.Meta{Pid: "a", Nonce: 1, Mode: schema.ExecModeDryRun})
	suite.wait(vm.entered)
	accepted := make(chan struct{})
	go func() {
		for i := 2; i <= count; i++ {
			v.Apply(schema.Meta{Pid: "a", Nonce: int64(i), Mode: schema.ExecModeDryRun})
		}
		close(accepted)
	}()
	suite.wait(accepted)
	finished := make(chan struct{})
	go func() {
		v.Apply(schema.Meta{Pid: "b", Nonce: 1, Mode: schema.ExecModeDryRun})
		_, _ = v.Checkpoint("b")
		close(finished)
	}()
	suite.wait(finished)
	unblock()
	snapshot, err := v.Checkpoint("a")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(count), snapshot.Env.Nonce)
	require.Len(suite.T(), vm.nonces, count)
	for i, nonce := range vm.nonces {
		assert.Equal(suite.T(), int64(i+1), nonce)
	}
}

func (suite *VmmQueueTestSuite) TestCloseStartsOtherPIDBeforeBlockedApplyFinishes() {
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	v := New(nil, &nodeSchema.Info{}, make(chan schema.VmmResult, 4), make(chan schema.Outbox, 4), nil)
	a := &queueTestVM{entered: make(chan struct{}), release: release, closed: make(chan struct{})}
	b := &queueTestVM{closed: make(chan struct{})}
	v.addVm(a, &schema.Env{Meta: schema.Meta{Pid: "a"}})
	v.addVm(b, &schema.Env{Meta: schema.Meta{Pid: "b"}})
	suite.T().Cleanup(func() { unblock(); v.Close() })
	v.Apply(schema.Meta{Pid: "a", Nonce: 1, Mode: schema.ExecModeDryRun})
	suite.wait(a.entered)
	done := make(chan struct{})
	go func() { v.Close(); close(done) }()
	suite.wait(b.closed)
	select {
	case <-a.closed:
		assert.Fail(suite.T(), "A closed during Apply")
	default:
	}
	unblock()
	suite.wait(done)
	assert.Equal(suite.T(), []int64{1}, a.nonces)
}

func (suite *VmmQueueTestSuite) TestCheckpointEnvironmentIsIndependent() {
	v := New(nil, &nodeSchema.Info{}, make(chan schema.VmmResult, 4), make(chan schema.Outbox, 4), nil)
	suite.T().Cleanup(v.Close)
	env := schema.Env{Meta: schema.Meta{Pid: "a", Params: map[string]string{"key": "original"}}, ReceivedSeq: map[string]int64{"sender": 1}}
	v.addVm(&queueTestVM{}, &env)
	env.Meta.Params["key"] = "changed"
	snapshot, err := v.Checkpoint("a")
	require.NoError(suite.T(), err)
	snapshot.Env.ReceivedSeq["sender"] = 99
	snapshot.Env.Meta.Params["key"] = "snapshot changed"
	_, current, err := v.GetVm("a")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "original", current.Meta.Params["key"])
	assert.Equal(suite.T(), int64(1), current.ReceivedSeq["sender"])
}

func TestVmmQueueTestSuite(t *testing.T) { suite.Run(t, new(VmmQueueTestSuite)) }

func (suite *VmmQueueTestSuite) TestFailedCreationCanRetry() {
	v := New(nil, &nodeSchema.Info{}, make(chan schema.VmmResult, 4), make(chan schema.Outbox, 4), nil)
	suite.T().Cleanup(v.Close)
	meta := schema.Meta{Pid: "a", Mode: schema.ExecModeDryRun}
	module := hySchema.Module{ModuleFormat: schema.ModuleFormatToken}
	assert.ErrorIs(suite.T(), v.Spawn(meta, hySchema.Process{}, module), schema.ErrInvalidModuleFormat)
	assert.Empty(suite.T(), v.vms)
	require.NoError(suite.T(), v.Mount(module.ModuleFormat, func(schema.Env) (schema.Vm, error) { return &queueTestVM{}, nil }))
	require.NoError(suite.T(), v.Spawn(meta, hySchema.Process{}, module))
	assert.Equal(suite.T(), int64(1), v.GetVmCount())
}
