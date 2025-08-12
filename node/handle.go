package node

import (
	"errors"

	"github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func (n *Node) Handle(item goarSchema.BundleItem) (err error) {
	if err = goarUtils.VerifyBundleItem(item); err != nil {
		return
	}

	pid, accid, fromProcess, instance, err := utils.Decode(item)
	if err != nil {
		return
	}

	// VERY IMPORTANT!!!
	// Verify that the item is signed by the correct node owner.
	// If the accid is registered in the node list, it is allowed to send a From-Process message.
	// The From-Process value must also be registered under the corresponding node.
	// If this io a registration request sent to the registry, this step is skipped.
	if fromProcess != "" {
		if pid == n.vmm.RegistryId() {
			// Verify register process(spawned) message
			// For registry messages, the fromProcess should be the same as the process being registered
			// and the accid should be registered in the node list
			if err = n.verifyRegistryMessage(item, fromProcess); err != nil {
				log.Error("verify registry process message failed", "pid", pid, "accid", accid, "fromProcess", fromProcess)
				return
			}
		} else {
			if err = n.authNode(accid, fromProcess); err != nil {
				log.Error("auth node failed", "pid", pid, "accid", accid, "fromProcess", fromProcess)
				return
			}
		}
	}

	// check if process in recovering
	if n.vmm.IsRecovering(pid) {
		err = schema.ErrProcessIsRecovering
		return
	}

	// get nodes info, if need redirect
	isRedirect, nodes, err := n.isRedirect(pid)
	if err != nil {
		return err
	}

	switch v := instance.(type) {
	case hymxSchema.Process:
		// check if scheduler is not node accid
		if v.Scheduler != n.bundler.Address {
			err = schema.ErrRedirect
			log.Warn("handle process failed", "pid", pid, "scheduler", v.Scheduler, "nodeAccId", n.bundler.Address, "err", err)
			return
		}
		// check if process already register
		if len(nodes) != 0 {
			err = schema.ErrProcessAlreadyExists
			log.Error("handle process failed", "pid", pid, "err", err)
			return
		}

		// check if the process already exists before assignment
		// assigning to an invalid (already spawned) process may result in a non-contiguous nonce sequence.
		if n.vmm.IsExists(pid) {
			err = schema.ErrProcessAlreadyExists
			log.Error("handle process failed", "pid", pid, "err", err)
			return
		}

		n.assignProcChan <- schema.AssignProcess{
			Pid:     pid,
			AccId:   accid,
			Process: v,
			Item:    item,
		}
	case hymxSchema.Message:
		// check if need redirect
		if isRedirect {
			// todo: return nodes, schema.ErrRedirect
			err = schema.ErrRedirect
			log.Warn("handle message failed", "pid", pid, "err", err)
			return
		}

		// check if the process not found before assignment
		if !n.vmm.IsExists(pid) {
			err = schema.ErrProcessNotFound
			log.Error("handle message failed", "pid", pid, "err", err)
			return
		}

		n.assignMesChan <- schema.AssignMessage{
			Pid:     pid,
			AccId:   accid,
			Message: v,
			Item:    item,
		}
	default:
		err = schema.ErrInvalidType
	}

	return
}

func (n *Node) HandleDryRun(item goarSchema.BundleItem, assign hymxSchema.Assignment, maxNonce int64) (err error) {
	pid, accid, _, instance, err := utils.Decode(item)
	if err != nil {
		return
	}

	switch v := instance.(type) {
	case hymxSchema.Process:
		return n.handleProcess(pid, accid, item, v, true, maxNonce)
	case hymxSchema.Message:
		return n.handleMessage(pid, accid, item, v, assign, true, maxNonce)
	default:
		return schema.ErrInvalidType
	}
}

func (n *Node) isRedirect(pid string) (ok bool, nodes []registrySchema.Node, err error) {
	ok = true
	nodes, err = n.vmm.GetNodesByProcess(pid)
	if err != nil {
		return
	}

	for _, node := range nodes {
		if node.AccId == n.bundler.Address {
			ok = false
			return
		}
	}
	return
}

func (n *Node) authNode(accid, fromProcess string) (err error) {
	nodes, err := n.GetNodesByProcess(fromProcess)
	if err != nil {
		return err
	}

	validNode := false
	for _, node := range nodes {
		if node.AccId == accid {
			validNode = true
			return
		}
	}
	if !validNode {
		log.Error("auth node failed", "accid", accid, "fromProcess", fromProcess)
		return schema.ErrUnauthorizedNode
	}

	return
}

// verifyRegistryMessage verifies registry process registration messages with 4 steps
func (n *Node) verifyRegistryMessage(item goarSchema.BundleItem, fromProcess string) (err error) {
	// 1. get 'Pid' and 'Acc-Id' from Tags
	pid := utils.GetTagsValue("Pid", item.Tags)
	accid := utils.GetTagsValue("Acc-Id", item.Tags)
	action := utils.GetTagsValue("Action", item.Tags)

	// Verify this is a RegisterProcess action
	if action != "RegisterProcess" {
		return schema.ErrInvalidType
	}

	if pid == "" || accid == "" {
		log.Error("missing required tags in registry message", "Pid", pid, "Acc-Id", accid)
		return schema.ErrUnauthorizedNode
	}

	// 2. get original message by 'Pid'
	// * pid is the id of the spawn message that created the process
	// * so we can use pid to query the original message
	spawnMsg, err := n.GetMessage(pid)
	if err != nil {
		return errors.New("get origin spawn message failed, msgid: " + pid)
	}
	if spawnMsg == nil {
		return errors.New("spawn message not found, msgid: " + pid)
	}

	// 3. verify original message
	if err = goarUtils.VerifyBundleItem(*spawnMsg); err != nil {
		return
	}

	// 4. Validate that the spawn message's scheduler matches the node via RegisterProcess.​
	scheduler := utils.GetTagsValue("Scheduler", spawnMsg.Tags)
	if accid != scheduler {
		log.Error("verify registry message failed, node accid not match", "pid", pid, "accid", accid, "fromProcess", fromProcess, "scheduler", scheduler)
		return schema.ErrUnauthorizedNode
	}

	return
}
