package node

import (
	"github.com/hymatrix/hymx/node/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func (n *Node) Stop(pid string) error {
	return n.stop(pid, false)
}

// StopWithCheckpoint saves the VM before stopping it. A save failure leaves it running.
func (n *Node) StopWithCheckpoint(pid string) error {
	return n.stop(pid, true)
}

func (n *Node) stop(pid string, checkpoint bool) error {
	if n.isCore(pid) {
		return schema.ErrCoreProcessCannotStop
	}
	if n.vmm.IsRecovering(pid) {
		return schema.ErrProcessIsRecovering
	}
	if registered, _ := n.isRegistered(pid); !registered {
		return schema.ErrProcessNotFound
	}
	if !n.vmm.IsExists(pid) {
		return schema.ErrProcessStopped
	}

	if checkpoint {
		if _, err := n.SaveCheckpoint(pid); err != nil {
			return err
		}
	}
	return n.vmm.Kill(pid)
}

func (n *Node) Resume(pid string) error {
	if n.vmm.IsExists(pid) {
		return schema.ErrProcessAlreadyExists
	}

	registered, err := n.isRegistered(pid)
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

func (n *Node) Running() []string {
	return n.vmm.GetVmPids()
}

func (n *Node) isCore(pid string) bool {
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

func (n *Node) isRegistered(pid string) (bool, error) {
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
