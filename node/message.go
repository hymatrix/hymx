package node

import (
	"strconv"

	"github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (n *Node) handleMessage(
	pid, accid string, item goarSchema.BundleItem,
	msg hymxSchema.Message,
) (err error) {
	// check if need redirect
	isRedirect, _, err := n.IsRedirect(pid)
	if err != nil {
		return
	}
	if isRedirect {
		err = schema.ErrMessageRedirect
		log.Warn("message redirect", "pid", pid, "err", err)
		return
	}

	// check if the process not found before assignment
	if registered, _ := n.isRegistered(pid); registered {
		err = schema.ErrProcessNotFound
		log.Error("handle message failed", "pid", pid, "err", err)
		return
	}

	if !n.vmm.IsExists(pid) {
		err = schema.ErrProcessStopped
		log.Error("handle message failed", "pid", pid, "err", err)
		return
	}

	n.assignMesChan <- schema.AssignMessage{
		Pid:     pid,
		AccId:   accid,
		Message: msg,
		Item:    item,
	}
	return nil
}

func (n *Node) applyMessage(
	pid, accid string,
	item goarSchema.BundleItem, msg hymxSchema.Message,
	assign hymxSchema.Assignment, mode vmmSchema.ExecMode, maxNonce int64,
) (err error) {
	nonce, err := strconv.ParseInt(assign.Nonce, 10, 64)
	if err != nil {
		return
	}
	timestamp, err := strconv.ParseInt(assign.Timestamp, 10, 64)
	if err != nil {
		return
	}

	params, err := utils.TagsToParams(msg.Tags)
	if err != nil {
		return err
	}

	sequence := int64(0)
	if msg.Sequence != "" {
		sequence, err = strconv.ParseInt(msg.Sequence, 10, 64)
		if err != nil {
			return
		}
	}

	n.vmm.Apply(vmmSchema.Meta{
		ItemId:           item.Id,
		Pid:              pid,
		AccId:            accid,
		Action:           msg.Action,
		FromProcess:      msg.FromProcess,
		PushedFor:        msg.PushedFor,
		Sequence:         sequence,
		Nonce:            nonce,
		Timestamp:        timestamp,
		Params:           params,
		Data:             item.Data,
		Mode:             mode,
		RecoveryMaxNonce: maxNonce,
	})
	return
}
