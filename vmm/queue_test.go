package vmm

import (
	"context"
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

// Done signals that submit has reached the channel send/cancellation select.
type queueTestContext struct {
	context.Context
	waiting chan struct{}
	release <-chan struct{}
}

func (ctx *queueTestContext) Done() <-chan struct{} {
	select {
	case ctx.waiting <- struct{}{}:
	default:
	}
	if ctx.release != nil {
		<-ctx.release
	}
	return ctx.Context.Done()
}

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

func (suite *VmmQueueTestSuite) TestApplyBlocksWhenFullAndPreservesFIFO() {
	const count = schema.VmQueueCapacity + 2
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	v := New(nil, &nodeSchema.Info{}, make(chan schema.VmmResult, count+4), make(chan schema.Outbox, count+4), nil)
	ctx := &queueTestContext{Context: v.ctx, waiting: make(chan struct{}, 1)}
	v.ctx = ctx
	vm := &queueTestVM{entered: make(chan struct{}), release: release}
	v.addVm(vm, &schema.Env{Meta: schema.Meta{Pid: "a"}})
	v.addVm(&queueTestVM{}, &schema.Env{Meta: schema.Meta{Pid: "b"}})
	suite.T().Cleanup(func() { unblock(); v.Close() })
	v.Apply(schema.Meta{Pid: "a", Nonce: 1, Mode: schema.ExecModeDryRun})
	suite.wait(vm.entered)
	for i := 2; i < count; i++ {
		v.Apply(schema.Meta{Pid: "a", Nonce: int64(i), Mode: schema.ExecModeDryRun})
	}
	suite.wait(ctx.waiting) // Discard notifications from the completed sends.
	accepted := make(chan struct{})
	go func() {
		v.Apply(schema.Meta{Pid: "a", Nonce: int64(count), Mode: schema.ExecModeDryRun})
		close(accepted)
	}()
	suite.T().Cleanup(func() { unblock(); suite.wait(accepted) })
	suite.wait(ctx.waiting)
	instance, err := v.instance("a", false)
	require.NoError(suite.T(), err)
	instance.Mu.Lock()
	assert.Len(suite.T(), instance.Tasks, schema.VmQueueCapacity)
	instance.Mu.Unlock()
	select {
	case <-accepted:
		assert.Fail(suite.T(), "Apply returned while the queue was full")
	default:
	}
	finished := make(chan struct{})
	go func() {
		v.Apply(schema.Meta{Pid: "b", Nonce: 1, Mode: schema.ExecModeDryRun})
		_, _ = v.Checkpoint("b")
		close(finished)
	}()
	suite.T().Cleanup(func() { unblock(); suite.wait(finished) })
	suite.wait(finished)
	unblock()
	suite.wait(accepted)
	snapshot, err := v.Checkpoint("a")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(count), snapshot.Env.Nonce)
	require.Len(suite.T(), vm.nonces, count)
	for i, nonce := range vm.nonces {
		assert.Equal(suite.T(), int64(i+1), nonce)
	}
}

func (suite *VmmQueueTestSuite) TestFullQueueWaitersWakeOnStop() {
	for _, closeVmm := range []bool{false, true} {
		name := "Kill"
		if closeVmm {
			name = "Close"
		}
		suite.Run(name, func() {
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			v := New(nil, &nodeSchema.Info{}, make(chan schema.VmmResult, schema.VmQueueCapacity+1), nil, nil)
			ctx := &queueTestContext{Context: v.ctx, waiting: make(chan struct{}, 1)}
			v.ctx = ctx
			vm := &queueTestVM{entered: make(chan struct{}), release: release}
			v.addVm(vm, &schema.Env{Meta: schema.Meta{Pid: "a"}})
			suite.T().Cleanup(func() { unblock(); v.Close() })
			v.Apply(schema.Meta{Pid: "a", Nonce: 1, Mode: schema.ExecModeDryRun})
			suite.wait(vm.entered)
			for i := 2; i <= schema.VmQueueCapacity+1; i++ {
				v.Apply(schema.Meta{Pid: "a", Nonce: int64(i), Mode: schema.ExecModeDryRun})
			}
			suite.wait(ctx.waiting)
			// Multiple callers must all wake, even though the worker cannot dequeue.
			replies := make(chan error, 2)
			for i := 0; i < cap(replies); i++ {
				go func() {
					_, err := v.Checkpoint("a")
					replies <- err
				}()
				suite.wait(ctx.waiting)
			}
			stopped := make(chan struct{})
			stopErr := make(chan error, 1)
			go func() {
				if closeVmm {
					v.Close()
					stopErr <- nil
				} else {
					stopErr <- v.Kill("a")
				}
				close(stopped)
			}()
			suite.T().Cleanup(func() { unblock(); suite.wait(stopped) })
			expected := schema.ErrVmStopping
			if closeVmm {
				expected = schema.ErrVmmClosed
			}
			for i := 0; i < cap(replies); i++ {
				select {
				case err := <-replies:
					assert.ErrorIs(suite.T(), err, expected)
				case <-time.After(5 * time.Second):
					require.FailNow(suite.T(), "queue waiter did not stop")
				}
			}
			unblock()
			suite.wait(stopped)
			assert.NoError(suite.T(), <-stopErr)
			assert.Len(suite.T(), vm.nonces, schema.VmQueueCapacity+1)
		})
	}
}

func (suite *VmmQueueTestSuite) TestStopWaitsForConcurrentAdmission() {
	v := New(nil, &nodeSchema.Info{}, nil, nil, nil)
	v.addVm(&queueTestVM{}, &schema.Env{Meta: schema.Meta{Pid: "a"}})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	ctx := &queueTestContext{Context: v.ctx, waiting: make(chan struct{}, 1), release: release}
	v.ctx = ctx
	suite.T().Cleanup(func() { unblock(); v.Close() })
	reply := make(chan error, 1)
	go func() {
		_, err := v.Checkpoint("a")
		reply <- err
	}()
	suite.wait(ctx.waiting)
	instance, err := v.instance("a", false)
	require.NoError(suite.T(), err)
	stop := v.stop("a", instance)
	unblock()
	select {
	case err := <-reply:
		// Both select outcomes are valid: reject the task, or execute it before stop.
		if err != nil {
			assert.ErrorIs(suite.T(), err, schema.ErrVmStopping)
		}
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "concurrent request was stranded after stop")
	}
	suite.wait(stop.Done)
	assert.NoError(suite.T(), stop.Err)
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
