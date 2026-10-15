package node

import (
	"fmt"

	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func (n *Node) runReplay() error {
	// get all process id and nonce from db first
	var allProcessId []string
	var allNonce []int64
	var err error

	allProcessId, allNonce, err = n.db.GetAllProcess()
	if err != nil || len(allProcessId) == 0 {
		allProcessId, err = n.sdk.Client.GetProcesses(n.info.Node.AccId)
		if err != nil {
			return err
		}
		allNonce = nil
	}
	return n.recoverProcesses(allProcessId, allNonce, vmmSchema.ExecModeReplay)
}

func (n *Node) getMaxNonce(pid string) (nonce int64, err error) {
	if n.chainkit == nil {
		return 0, fmt.Errorf("chainkit is nil")
	}
	return n.chainkit.GetMaxNonce(n.info.Node.AccId, pid)
}
