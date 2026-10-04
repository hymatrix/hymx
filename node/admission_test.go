package node

import (
	"sync"
	"time"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type admissionTestVM struct {
	lifecycleVM
	firstEntered  chan struct{}
	firstRelease  chan struct{}
	secondEntered chan struct{}
	secondRelease chan struct{}
}

func (vm *admissionTestVM) Apply(_ string, meta vmmSchema.Meta) vmmSchema.Result {
	switch meta.Nonce {
	case 1:
		close(vm.firstEntered)
		<-vm.firstRelease
	case 2:
		close(vm.secondEntered)
		<-vm.secondRelease
	}
	return vmmSchema.Result{}
}

func (suite *NodeVMLifecycleTestSuite) TestMessageAdmissionChecksVMQueueAndResumesAfterDequeue() {
	vm := &admissionTestVM{
		firstEntered: make(chan struct{}), firstRelease: make(chan struct{}),
		secondEntered: make(chan struct{}), secondRelease: make(chan struct{}),
	}
	n := suite.newLifecycleNode("a", vm, &lifecycleDB{})
	var firstOnce, secondOnce sync.Once
	releaseFirst := func() { firstOnce.Do(func() { close(vm.firstRelease) }) }
	suite.T().Cleanup(func() {
		releaseFirst()
		secondOnce.Do(func() { close(vm.secondRelease) })
	})
	n.info.Node.AccId = n.bundler.Address
	suite.registerProcess(n, "a")
	n.assignMesChan = make(chan nodeSchema.AssignMessage, 1)
	wait := func(done <-chan struct{}) {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			require.FailNow(suite.T(), "operation timed out")
		}
	}
	checkAdmission := func(pid string, expected error) {
		reply := make(chan error, 1)
		go func() {
			reply <- n.handleMessage(pid, "accid", goarSchema.BundleItem{Id: "message"}, hymxSchema.Message{})
		}()
		select {
		case err := <-reply:
			if expected != nil {
				assert.ErrorIs(suite.T(), err, expected)
				assert.Empty(suite.T(), n.assignMesChan)
			} else {
				require.NoError(suite.T(), err)
				require.Len(suite.T(), n.assignMesChan, 1)
				assert.Equal(suite.T(), pid, (<-n.assignMesChan).Pid)
			}
		case <-time.After(5 * time.Second):
			require.FailNow(suite.T(), "message admission blocked")
		}
	}
	require.Less(suite.T(), nodeSchema.VmPendingMessageLimit, vmmSchema.VmQueueCapacity)
	_, err := n.vmm.GetVmQueueLength("missing")
	assert.ErrorIs(suite.T(), err, vmmSchema.ErrProcessNotFound)
	n.vmm.Apply(vmmSchema.Meta{Pid: "a", Nonce: 1})
	wait(vm.firstEntered)
	for nonce := 2; nonce <= nodeSchema.VmPendingMessageLimit; nonce++ {
		n.vmm.Apply(vmmSchema.Meta{Pid: "a", Nonce: int64(nonce)})
	}
	pending, err := n.vmm.GetVmQueueLength("a")
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), nodeSchema.VmPendingMessageLimit-1, pending)
	checkAdmission("a", nil)
	n.vmm.Apply(vmmSchema.Meta{Pid: "a", Nonce: nodeSchema.VmPendingMessageLimit + 1})
	checkAdmission("a", nodeSchema.ErrProcessBusy)
	// Dequeuing one task reopens admission even while that task is executing.
	releaseFirst()
	wait(vm.secondEntered)
	checkAdmission("a", nil)
	n.vmm.Apply(vmmSchema.Meta{Pid: "a", Nonce: nodeSchema.VmPendingMessageLimit + 2})
	checkAdmission("a", nodeSchema.ErrProcessBusy)
	require.NoError(suite.T(), n.vmm.Mount("other", func(vmmSchema.Env) (vmmSchema.Vm, error) {
		return &lifecycleVM{}, nil
	}))
	require.NoError(suite.T(), n.vmm.Restore(vmmSchema.Snapshot{Env: vmmSchema.Env{
		Meta: vmmSchema.Meta{Pid: "b"}, Module: hymxSchema.Module{ModuleFormat: "other"},
	}}))
	suite.registerProcess(n, "b")
	checkAdmission("b", nil)
}
