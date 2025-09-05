package node

import (
	"strconv"

	hymxSchem "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (n *Node) handleMessage(
	pid, accid string,
	item goarSchema.BundleItem, msg hymxSchem.Message,
	assign hymxSchem.Assignment, dryRun bool, maxNonce int64,
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

	n.vmm.Apply(schema.Meta{
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
		DryRun:           dryRun,
		RecoveryMaxNonce: maxNonce,
	})
	return
}
