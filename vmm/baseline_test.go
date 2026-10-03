package vmm

import (
	"sync"
	"testing"
	"time"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/vmm/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// VmmBaselineTestSuite tests the current global queue blocking behavior.
// Replace these assertions with cross-PID progress checks during P2.
type VmmBaselineTestSuite struct {
	suite.Suite
}

type baselineTestVM struct {
	applyEntered      chan struct{}
	checkpointEntered chan struct{}
	release           <-chan struct{}
	blockApply        bool
}

func (v *baselineTestVM) Apply(from string, meta schema.Meta) schema.Result {
	close(v.applyEntered)
	if v.blockApply && v.release != nil {
		<-v.release
	}
	return schema.Result{}
}

func (v *baselineTestVM) Checkpoint() (string, error) {
	close(v.checkpointEntered)
	if !v.blockApply && v.release != nil {
		<-v.release
	}
	return "{}", nil
}

func (v *baselineTestVM) Restore(data string) error { return nil }
func (v *baselineTestVM) Close() error              { return nil }

func (suite *VmmBaselineTestSuite) waitForOperation(ch <-chan struct{}) {
	suite.T().Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "operation did not complete")
	}
}

func (suite *VmmBaselineTestSuite) checkGlobalQueueBlocking(blockApply bool) {
	suite.T().Helper()

	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	v := New(nil, &nodeSchema.Info{}, make(chan schema.VmmResult, 4), make(chan schema.Outbox, 4), nil)
	a := &baselineTestVM{
		applyEntered:      make(chan struct{}),
		checkpointEntered: make(chan struct{}),
		release:           release,
		blockApply:        blockApply,
	}
	b := &baselineTestVM{
		applyEntered:      make(chan struct{}),
		checkpointEntered: make(chan struct{}),
	}
	for pid, vm := range map[string]*baselineTestVM{"pid-a": a, "pid-b": b} {
		v.addVm(vm, &schema.Env{
			Meta:        schema.Meta{Pid: pid},
			ReceivedSeq: map[string]int64{},
		})
	}
	v.Run()
	suite.T().Cleanup(func() {
		// Always release the VM before waiting for VMM shutdown.
		unblock()
		v.Close()
	})

	if blockApply {
		v.Apply(schema.Meta{Pid: "pid-a", Nonce: 1, Mode: schema.ExecModeDryRun})
		suite.waitForOperation(a.applyEntered)
	} else {
		v.ckpChan <- schema.Checkpoint{Pid: "pid-a", Res: make(chan schema.Snapshot, 1)}
		suite.waitForOperation(a.checkpointEntered)
	}

	v.Apply(schema.Meta{Pid: "pid-b", Nonce: 1, Mode: schema.ExecModeDryRun})
	reply := make(chan schema.Snapshot, 1)
	v.ckpChan <- schema.Checkpoint{Pid: "pid-b", Res: reply}

	// A cannot return until released; both B operations remain queued.
	assert.Len(suite.T(), v.applyChan, 1)
	assert.Len(suite.T(), v.ckpChan, 1)
	select {
	case <-b.applyEntered:
		assert.Fail(suite.T(), "B executed while A was blocked")
	default:
	}
	select {
	case <-b.checkpointEntered:
		assert.Fail(suite.T(), "B checkpoint executed while A was blocked")
	default:
	}

	unblock()
	suite.waitForOperation(b.applyEntered)
	suite.waitForOperation(b.checkpointEntered)
	select {
	case snap := <-reply:
		assert.NoError(suite.T(), snap.Err)
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "checkpoint reply missing")
	}
}

func (suite *VmmBaselineTestSuite) TestApplyBlocksOtherVMOperations() {
	suite.checkGlobalQueueBlocking(true)
}

func (suite *VmmBaselineTestSuite) TestCheckpointBlocksOtherVMOperations() {
	suite.checkGlobalQueueBlocking(false)
}

func TestVmmBaselineTestSuite(t *testing.T) {
	suite.Run(t, new(VmmBaselineTestSuite))
}
