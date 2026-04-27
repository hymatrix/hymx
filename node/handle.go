package node

import (
	"errors"

	"github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/hymatrix/hymx/utils"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func (n *Node) Handle(item goarSchema.BundleItem) (err error) {
	if err = goarUtils.VerifyBundleItem(item); err != nil {
		return
	}

	pid, signer, fromProcess, instance, err := utils.Decode(item)
	if err != nil {
		return
	}

	// Custom handler to process and filter items
	n.itemHandlerLockMu.RLock()
	for _, handler := range n.itemHandlers {
		if err = handler(schema.ItemMeta{
			Pid:         pid,
			Signer:      signer,
			FromProcess: fromProcess,
			Instance:    instance,
		}); err != nil {
			n.itemHandlerLockMu.RUnlock()
			return
		}
	}
	n.itemHandlerLockMu.RUnlock()

	// VERY IMPORTANT!!!
	// Verify that the item is signed by the correct node owner.
	// If the accid is registered in the node list, it is allowed to send a From-Process message.
	// The From-Process value must also be registered under the corresponding node.
	// If this io a registration request sent to the registry, this step is skipped.
	if err = n.verifyFromProcess(item, pid, signer, fromProcess); err != nil {
		return
	}

	// check if process in recovering
	if n.vmm.IsRecovering(pid) {
		err = schema.ErrProcessIsRecovering
		return
	}

	switch v := instance.(type) {
	case hymxSchema.Process:
		if v.Scheduler != n.bundler.Address {
			return n.handleProcess(pid, signer, item, v)
		}
	case hymxSchema.Message:
		isRedirect, _, err := n.IsRedirect(pid)
		if err != nil {
			return err
		}
		if isRedirect || !n.vmm.IsExists(pid) {
			return n.handleMessage(pid, signer, item, v)
		}
	}

	_, internalInstance, err := n.decryptInternalItem(item)
	if err != nil {
		return
	}

	switch v := internalInstance.(type) {
	case hymxSchema.Process:
		err = n.handleProcess(pid, signer, item, v)
	case hymxSchema.Message:
		err = n.handleMessage(pid, signer, item, v)
	default:
		err = schema.ErrInvalidType
	}

	return
}

func (n *Node) HandleMode(item goarSchema.BundleItem, assign hymxSchema.Assignment, mode vmmSchema.ExecMode, maxNonce int64) (err error) {
	pid, accid, _, instance, err := n.decodeInternalItem(item)
	if err != nil {
		return
	}

	switch v := instance.(type) {
	case hymxSchema.Process:
		return n.applyProcess(pid, accid, item, v, mode, maxNonce)
	case hymxSchema.Message:
		return n.applyMessage(pid, accid, item, v, assign, mode, maxNonce)
	default:
		return schema.ErrInvalidType
	}
}

// verifyFromProcess verifies the fromProcess authentication
// It handles both registry process verification and regular node authentication
func (n *Node) verifyFromProcess(item goarSchema.BundleItem, pid, signer, fromProcess string) error {
	if fromProcess == "" {
		return nil
	}

	// Handle registry process verification
	if pid == n.vmm.RegistryId() {
		// return n.verifyRegistryProcess(item, pid, signer, fromProcess)
		// Verify this is a RegisterProcess action
		action := utils.GetTagsValue("Action", item.Tags)
		if action != "RegisterProcess" {
			return nil
		}

		if err := n.verifyRegistry(item, signer, fromProcess); err != nil {
			log.Error("verify registry process message failed", "pid", pid, "signer", signer, "fromProcess", fromProcess, "err", err)
			return err
		}
		return nil
	}

	// Handle regular node authentication
	if err := n.authNode(signer, fromProcess); err != nil {
		log.Error("auth node failed", "pid", pid, "signer", signer, "fromProcess", fromProcess)
		return err
	}

	return nil
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

// verifyRegistry verifies registry process registration messages with 4 steps
func (n *Node) verifyRegistry(item goarSchema.BundleItem, signer, fromProcess string) (err error) {
	// 1. get 'Pid' and 'Acc-Id' from Tags
	pid := utils.GetTagsValue("Pid", item.Tags)
	accid := utils.GetTagsValue("Acc-Id", item.Tags)
	if pid == "" || accid == "" {
		log.Error("missing required tags in registry message", "Pid", pid, "Acc-Id", accid)
		return schema.ErrUnauthorizedNode
	}

	// 2. get original message by 'Pid'
	// * pid is the id of the spawn message that created the process
	// * so we can use pid to query the original message
	var spawnMsg *goarSchema.BundleItem
	if signer == n.Info().Node.AccId { // get message from local
		spawnMsg, err = n.GetMessage(pid)
		if err != nil {
			return errors.New("get origin spawn message failed, msgid: " + pid)
		}
	} else { // get message from other node
		var node *registrySchema.Node
		node, err = n.GetNode(signer)
		if err != nil {
			return
		}
		if node == nil {
			return errors.New("node not found, accid: " + signer)
		}
		cli := sdk.NewClient(node.URL)
		msg, msgErr := cli.GetMessage(pid)
		if msgErr != nil {
			return errors.New("get origin spawn message failed, msgid: " + pid)
		}
		spawnMsg = &msg
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
