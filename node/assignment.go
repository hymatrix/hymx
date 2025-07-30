package node

import (
	"fmt"
	"time"

	"github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	goarSchema "github.com/permadao/goar/schema"
)

func (n *Node) assignment(pid string, item goarSchema.BundleItem) (assign hymxSchema.Assignment, assignItem goarSchema.BundleItem, err error) {
	msg, _ := n.db.GetMessage(item.Id)
	if msg != nil {
		err = schema.ErrDuplicateItem
		return
	}

	nonce := int64(0)
	nonce, err = n.db.GetNonce(pid)
	if err != nil {
		log.Error("assignment get nonce failed", "pid", pid, "err", err)
		return
	}
	nonce = nonce + 1

	assign, assignItem, err = n.signAssign(pid, item.Id, nonce)
	if err != nil {
		log.Error("assignment sign assign failed", "pid", pid, "err", err)
		return
	}

	err = n.db.Commit(pid, nonce, item, assignItem)
	if err != nil {
		log.Error("assignment commit failed", "pid", pid, "err", err)
		return
	}
	return
}

func (n *Node) signAssign(pid, msgid string, nonce int64) (assign hymxSchema.Assignment, assignItem goarSchema.BundleItem, err error) {
	assign = hymxSchema.Assignment{
		Base:      hymxSchema.DefaultBaseAssignment,
		Process:   pid,
		Message:   msgid,
		Nonce:     fmt.Sprintf("%d", nonce),
		Timestamp: fmt.Sprintf("%d", time.Now().UnixMilli()),
	}
	tags, err := utils.AssignmentToTags(assign)
	if err != nil {
		return
	}
	assignItem, err = n.bundler.CreateAndSignItem([]byte{}, "", "", tags)
	return
}
