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

// VmmBaselineTestSuite tests cross-PID execution isolation.
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

func (suite *VmmBaselineTestSuite) checkIndependentProgress(blockApply bool) {
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
		go func() { _, _ = v.Checkpoint("pid-a") }()
		suite.waitForOperation(a.checkpointEntered)
	}

	v.Apply(schema.Meta{Pid: "pid-b", Nonce: 1, Mode: schema.ExecModeDryRun})
	reply := make(chan error, 1)
	go func() { _, err := v.Checkpoint("pid-b"); reply <- err }()
	suite.waitForOperation(b.applyEntered)
	suite.waitForOperation(b.checkpointEntered)
	select {
	case err := <-reply:
		assert.NoError(suite.T(), err)
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "B checkpoint blocked behind A")
	}
	unblock()

}

func (suite *VmmBaselineTestSuite) TestApplyDoesNotBlockOtherVMOperations() {
	suite.checkIndependentProgress(true)
}

func (suite *VmmBaselineTestSuite) TestCheckpointDoesNotBlockOtherVMOperations() {
	suite.checkIndependentProgress(false)
}

func TestVmmBaselineTestSuite(t *testing.T) {
	suite.Run(t, new(VmmBaselineTestSuite))
}
