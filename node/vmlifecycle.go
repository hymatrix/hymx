package node

import (
	"github.com/hymatrix/hymx/node/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func (n *Node) processRegisteredToLocalNode(pid string) (bool, error) {
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

	ckpItem, err := n.Checkpoint(pid)
	if err != nil {
		return err
	}
	if err = SaveCheckpoint(ckpItem); err != nil {
		return err
	}
	if err = n.db.SaveCheckpointIndex(pid, ckpItem.Id); err != nil {
		return err
	}

	if err = n.vmm.Kill(pid); err != nil {
		_ = n.restoreAfterFailedStop(pid, ckpItem.Id)
		return err
	}
	return nil
}

func (n *Node) ResumeVM(pid string) error {
	if n.vmm.IsExists(pid) {
		return schema.ErrProcessAlreadyExists
	}

	registered, err := n.processRegisteredToLocalNode(pid)
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

	return n.recoveryProcess(pid, maxNonce, ckpId, vmmSchema.ExecModeDryRun)
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

func (n *Node) restoreAfterFailedStop(pid, ckpId string) error {
	maxNonce, err := n.db.GetNonce(pid)
	if err != nil {
		return err
	}
	return n.recoveryProcess(pid, maxNonce, ckpId, vmmSchema.ExecModeDryRun)
}
