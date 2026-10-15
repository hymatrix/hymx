package node

import (
	"errors"
	"fmt"
	"sort"

	"github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/utils"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (n *Node) runRecovery() error {
	pids, nonces, err := n.db.GetAllProcess()
	if err != nil {
		return err
	}
	return n.recoverProcesses(pids, nonces, vmmSchema.ExecModeDryRun)
}

type recoveryPlan struct {
	order         []int
	coreCount     int
	registryTasks map[int]bool
}

// Resolve metadata sequentially, retain valid jobs, and report failures by PID.
// Core tasks must finish before admitting ordinary VMs that depend on registry.
func (n *Node) recoveryOrder(pids []string) (recoveryPlan, error) {
	plan := recoveryPlan{registryTasks: make(map[int]bool)}
	priorities := make([]int, len(pids))
	type moduleResult struct {
		format string
		err    error
	}
	modules := make(map[string]moduleResult)
	var failures []error
	for i, pid := range pids {
		moduleID, err := n.recoveryModuleID(pid)
		priority := 2
		if err == nil {
			result, ok := modules[moduleID]
			if !ok {
				module, loadErr := n.LoadModule(moduleID)
				result = moduleResult{format: module.ModuleFormat, err: loadErr}
				modules[moduleID] = result
			}
			err = result.err
			switch result.format {
			case vmmSchema.ModuleFormatRegistry:
				priority = 0
			case vmmSchema.ModuleFormatToken:
				priority = 1
			}
		}
		if err != nil {
			failure := fmt.Errorf("skip recovery process %s (module %s): %w", pid, moduleID, err)
			log.Error("skipping recovery process", "pid", pid, "module", moduleID, "err", err)
			failures = append(failures, failure)
			continue
		}
		priorities[i] = priority
		plan.order = append(plan.order, i)
		if priority < 2 {
			plan.coreCount++
		}
		if priority == 0 {
			plan.registryTasks[i] = true
		}
	}
	sort.SliceStable(plan.order, func(i, j int) bool {
		return priorities[plan.order[i]] < priorities[plan.order[j]]
	})
	return plan, errors.Join(failures...)
}

func (n *Node) recoveryModuleID(pid string) (string, error) {
	msg, err := n.db.GetMessageByNonce(pid, 0)
	if err != nil || msg == nil {
		if n.sdk == nil {
			return "", fmt.Errorf("spawn message unavailable")
		}
		remote, err := n.sdk.Client.GetMessageByNonce(pid, 0)
		if err != nil {
			return "", err
		}
		msg = &remote
	}
	process, err := utils.TagsToProcess(msg.Tags)
	if err != nil {
		return "", err
	}
	if process.Module == "" {
		return "", fmt.Errorf("spawn message has no module")
	}
	return process.Module, nil
}

func (n *Node) recoveryProcess(pid string, maxNonce int64, ckpId string, mode vmmSchema.ExecMode) error {
	log.Debug("recovery process", "pid", pid, "maxNonce", maxNonce, "ckpId", ckpId, "mode", mode)
	if n.vmm.IsRecovering(pid) {
		return schema.ErrProcessIsRecovering
	}

	// lock process
	n.vmm.RecoveryLock(pid)

	// init nonce to 0
	beginNonce := int64(0)

	// load checkpoint if exist
	if nonce, err := n.Restore(ckpId); err == nil {
		if nonce == maxNonce {
			n.vmm.RecoveryUnlock(pid)
			return nil
		}

		beginNonce = nonce + 1
	}

	n.wg.Add(1)
	defer n.wg.Done()

	// recovering all message of the process
	for nonce := beginNonce; nonce <= maxNonce; nonce++ {
		select {
		case <-n.ctx.Done():
			return errors.New("node closed")
		default:
			msg, assignItem, err := n.getMessageAndAssignByNonce(pid, nonce)
			if err != nil {
				return err
			}

			assign, err := utils.TagsToAssignment(assignItem.Tags)
			if err != nil {
				return err
			}

			if err = n.HandleMode(*msg, assign, mode, maxNonce); err != nil {
				return fmt.Errorf("recover process %s at nonce %d: %w", pid, nonce, err)
			}
		}

	}
	return nil
}

// getMessageAndAssignByNonce retrieves both message and assignment by process ID and nonce
func (n *Node) getMessageAndAssignByNonce(pid string, nonce int64) (msgItem, assignItem *goarSchema.BundleItem, err error) {
	// First try to get from local database
	msgItem, err1 := n.db.GetMessageByNonce(pid, nonce)
	assignItem, err2 := n.db.GetAssignByNonce(pid, nonce)

	// If both are available locally, return them
	if err1 == nil && err2 == nil && msgItem != nil && assignItem != nil {
		return msgItem, assignItem, nil
	}

	// If either is missing, try to download once
	if n.chainkit == nil {
		return nil, nil, fmt.Errorf("chainkit is nil")
	}
	result, err := n.chainkit.DownloadByPid(pid, nonce, nonce)
	if err != nil {
		return nil, nil, err
	}
	if len(result) == 0 || result[0].Message == nil || result[0].Assignment == nil {
		return nil, nil, fmt.Errorf("message or assignment unavailable for process %s at nonce %d", pid, nonce)
	}
	msgItem = result[0].Message
	assignItem = result[0].Assignment

	return msgItem, assignItem, nil
}
