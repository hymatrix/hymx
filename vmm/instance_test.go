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

type VmmInstanceTestSuite struct{ suite.Suite }

type instanceTestVM struct {
	entered    chan struct{}
	release    chan struct{}
	closed     chan struct{}
	nonces     []int64
	restoreErr error
	restores   int
}

func (vm *instanceTestVM) Apply(_ string, meta schema.Meta) schema.Result {
	if len(vm.nonces) == 0 && vm.entered != nil {
		close(vm.entered)
		<-vm.release
	}
	vm.nonces = append(vm.nonces, meta.Nonce)
	return schema.Result{}
}
func (vm *instanceTestVM) Checkpoint() (string, error) { return "state", nil }
func (vm *instanceTestVM) Restore(string) error {
	vm.restores++
	return vm.restoreErr
}
func (vm *instanceTestVM) Close() error {
	if vm.closed != nil {
		close(vm.closed)
	}
	return nil
}

func (suite *VmmInstanceTestSuite) wait(done <-chan struct{}) {
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "timed out")
	}
}

func (suite *VmmInstanceTestSuite) newVmm() (*Vmm, chan schema.VmmResult) {
	results := make(chan schema.VmmResult, schema.VmQueueCapacity+8)
	v := New(nil, &nodeSchema.Info{}, results, nil, nil)
	suite.T().Cleanup(v.Close)
	return v, results
}

func (suite *VmmInstanceTestSuite) blockedVM(v *Vmm) (*instanceTestVM, func()) {
	vm := &instanceTestVM{entered: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
	var once sync.Once
	release := func() { once.Do(func() { close(vm.release) }) }
	suite.T().Cleanup(release)
	v.addVm(vm, &schema.Env{Meta: schema.Meta{Pid: "a"}})
	v.Apply(schema.Meta{Pid: "a", Nonce: 1})
	suite.wait(vm.entered)
	return vm, release
}

func (suite *VmmInstanceTestSuite) TestCapacityFIFOAndIndependentVM() {
	v, _ := suite.newVmm()
	a, release := suite.blockedVM(v)
	for nonce := 2; nonce <= schema.VmQueueCapacity+1; nonce++ {
		v.Apply(schema.Meta{Pid: "a", Nonce: int64(nonce)})
	}
	instance, err := v.getInstance("a")
	require.NoError(suite.T(), err)
	assert.Len(suite.T(), instance.Tasks, schema.VmQueueCapacity)
	started, sent := make(chan struct{}), make(chan struct{})
	go func() {
		close(started)
		v.Apply(schema.Meta{Pid: "a", Nonce: schema.VmQueueCapacity + 2})
		close(sent)
	}()
	suite.wait(started)
	select {
	case <-sent:
		require.FailNow(suite.T(), "full queue accepted another task")
	default:
	}
	b := &instanceTestVM{}
	v.addVm(b, &schema.Env{Meta: schema.Meta{Pid: "b"}})
	v.Apply(schema.Meta{Pid: "b", Nonce: 1})
	other := make(chan struct{})
	go func() {
		_, _ = v.Checkpoint("b")
		close(other)
	}()
	suite.wait(other)
	assert.Equal(suite.T(), []int64{1}, b.nonces)
	release()
	suite.wait(sent)
	require.NoError(suite.T(), v.Kill("a"))
	require.Len(suite.T(), a.nonces, schema.VmQueueCapacity+2)
	for i, nonce := range a.nonces {
		assert.Equal(suite.T(), int64(i+1), nonce)
	}
}

func (suite *VmmInstanceTestSuite) TestCheckpointAndRestoreAreSerialAndSnapshotsIndependent() {
	v, _ := suite.newVmm()
	_, release := suite.blockedVM(v)
	instance, err := v.getInstance("a")
	require.NoError(suite.T(), err)
	// A queued task gives a deterministic barrier behind Apply.
	barrier := &schema.VmTask{Done: make(chan struct{}), Run: func(instance *schema.VmInstance) error {
		instance.Env.Meta.Params = map[string]string{"key": "old"}
		instance.Env.ReceivedSeq["sender"] = 3
		return nil
	}}
	require.NoError(suite.T(), v.submit(instance, barrier))
	select {
	case <-barrier.Done:
		require.FailNow(suite.T(), "task overlapped Apply")
	default:
	}
	release()
	suite.wait(barrier.Done)
	snap, err := v.Checkpoint("a")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), snap.Env.Nonce)
	snap.Env.Meta.Params["key"] = "external"
	snap.Env.ReceivedSeq["sender"] = 99
	next, err := v.Checkpoint("a")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "old", next.Env.Meta.Params["key"])
	assert.Equal(suite.T(), int64(3), next.Env.ReceivedSeq["sender"])
	next.Env.Nonce = 10
	require.NoError(suite.T(), v.Restore(next))
	next.Env.ReceivedSeq["sender"] = 100
	v.Apply(schema.Meta{Pid: "a", Nonce: 11})
	final, err := v.Checkpoint("a")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(11), final.Env.Nonce)
	assert.Equal(suite.T(), int64(3), final.Env.ReceivedSeq["sender"])
}

func (suite *VmmInstanceTestSuite) TestStopWaitsForAdmittedSendAndDrainsFullQueue() {
	v, _ := suite.newVmm()
	vm, release := suite.blockedVM(v)
	instance, err := v.getInstance("a")
	require.NoError(suite.T(), err)
	for nonce := 2; nonce <= schema.VmQueueCapacity+1; nonce++ {
		v.Apply(schema.Meta{Pid: "a", Nonce: int64(nonce)})
	}
	// Reserve a sender exactly as submit does, then pause before its channel send.
	instance.Mu.Lock()
	instance.Submitters.Add(1)
	instance.Mu.Unlock()
	task := &schema.VmTask{Done: make(chan struct{}), Run: func(*schema.VmInstance) error { return nil }}
	sent := make(chan struct{})
	go func() {
		instance.Tasks <- task
		instance.Submitters.Done()
		close(sent)
	}()
	v.stopInstance(instance)
	assert.ErrorIs(suite.T(), v.call(instance, task.Run), schema.ErrVmStopping)
	select {
	case <-instance.Done:
		require.FailNow(suite.T(), "stop did not wait for running task")
	default:
	}
	release()
	suite.wait(sent)
	suite.wait(instance.Done)
	suite.wait(task.Done)
	assert.Len(suite.T(), vm.nonces, schema.VmQueueCapacity+1)
	assert.False(suite.T(), v.IsExists("a"))
}

func (suite *VmmInstanceTestSuite) TestCloseStopsAllInstancesBeforeWaiting() {
	v, _ := suite.newVmm()
	_, release := suite.blockedVM(v)
	b := &instanceTestVM{closed: make(chan struct{})}
	v.addVm(b, &schema.Env{Meta: schema.Meta{Pid: "b"}})
	done := make(chan struct{})
	go func() { v.Close(); close(done) }()
	suite.wait(b.closed)
	_, err := v.Checkpoint("a")
	assert.ErrorIs(suite.T(), err, schema.ErrVmmClosed)
	release()
	suite.wait(done)
	v.Close()
}

func (suite *VmmInstanceTestSuite) TestFailedCreationCanRetryAndClosesVM() {
	v, _ := suite.newVmm()
	failed := &instanceTestVM{restoreErr: errors.New("restore failed"), closed: make(chan struct{})}
	current := failed
	require.NoError(suite.T(), v.Mount("test", func(schema.Env) (schema.Vm, error) { return current, nil }))
	snap := schema.Snapshot{Env: schema.Env{Meta: schema.Meta{Pid: "a"}, Module: hymxSchema.Module{ModuleFormat: "test"}}}
	assert.Error(suite.T(), v.Restore(snap))
	suite.wait(failed.closed)
	assert.False(suite.T(), v.IsExists("a"))
	current = &instanceTestVM{}
	require.NoError(suite.T(), v.Restore(snap))
	assert.True(suite.T(), v.IsExists("a"))
}

func (suite *VmmInstanceTestSuite) TestConcurrentRestoreReusesReservedInstance() {
	v, _ := suite.newVmm()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	suite.T().Cleanup(unblock)
	vm := &instanceTestVM{}
	spawns := 0
	require.NoError(suite.T(), v.Mount("test", func(schema.Env) (schema.Vm, error) {
		spawns++
		close(entered)
		<-release
		return vm, nil
	}))
	snap := schema.Snapshot{Env: schema.Env{Meta: schema.Meta{Pid: "a"}, Module: hymxSchema.Module{ModuleFormat: "test"}}}
	first, second := make(chan error, 1), make(chan error, 1)
	go func() { first <- v.Restore(snap) }()
	suite.wait(entered)
	assert.True(suite.T(), v.IsExists("a"))
	go func() { second <- v.Restore(snap) }()
	unblock()
	for _, reply := range []chan error{first, second} {
		select {
		case err := <-reply:
			require.NoError(suite.T(), err)
		case <-time.After(5 * time.Second):
			require.FailNow(suite.T(), "restore timed out")
		}
	}
	assert.Equal(suite.T(), 1, spawns)
	assert.Equal(suite.T(), 2, vm.restores)
}

func TestVmmInstanceTestSuite(t *testing.T) {
	suite.Run(t, new(VmmInstanceTestSuite))
}
