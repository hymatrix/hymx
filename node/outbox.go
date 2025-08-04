package node

import (
	"fmt"
	"time"

	"github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/hymatrix/hymx/utils"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	vmmUtils "github.com/hymatrix/hymx/vmm/utils"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func (n *Node) outbox(outbox vmmSchema.Outbox) {
	accType, to, err := vmmUtils.IDCheck(outbox.To)
	if err != nil {
		log.Error("outbox invalid target accid", "err", err)
		return
	}

	target := to
	if accType != vmmSchema.AccountTypeAR {
		target = ""
	}
	tags := utils.MergeTags([]goarSchema.Tag{
		{Name: "SDK-Timestamp", Value: fmt.Sprintf("%d", time.Now().UnixNano())},
	}, outbox.Tags)

	item, err := n.bundler.CreateAndSignItem([]byte(outbox.Data), target, "", tags)
	if err != nil {
		log.Error("outbox sign item failed", "err", err)
		return
	}

	targetProcId := to
	if outbox.Type == hymxSchema.TypeProcess {
		targetProcId = item.Id
	}
	if err := n.outboxDB.Push(outbox.From, targetProcId, item); err != nil {
		log.Error("outbox push failed", "err", err)
		return
	}

	go n.trySend(outbox.From, targetProcId)
}

func (n *Node) trySend(pid, target string) {
	if n.isSending(pid, target) {
		return
	}

	n.sendingLock(pid, target)
	defer n.sendingUnlock(pid, target)
	for {
		_, item, err := n.outboxDB.Peek(pid, target)
		if err != nil {
			log.Error("outbox peek failed", "err", err)
			return
		}
		if item == nil {
			return
		}

		itemBin, err := goarUtils.GenerateItemBinary(*item)
		if err != nil {
			log.Error("outbox invalid item", "err", err)
			return
		}

		nodes := []registrySchema.Node{}
		switch utils.GetType(*item) {
		case hymxSchema.TypeMessage:
			nodes, err = n.vmm.GetNodesByProcess(target)
			if err != nil {
				log.Error("outbox send failed", "err", err)
				return
			}
			if len(nodes) == 0 {
				return
			}
		case hymxSchema.TypeProcess:
			scheduler := utils.GetTagsValue("Scheduler", item.Tags)
			node, err := n.vmm.GetNode(scheduler)
			if err != nil {
				log.Error("outbox spawn failed", "err", err)
				return
			}
			if node == nil {
				log.Error("outbox spawn failed, node not found", "node", target)
				return
			}

			nodes = append(nodes, *node)
		default:
			log.Error("outbox invalid type", "type", utils.GetType(*item))
			return
		}

		var assignItem goarSchema.BundleItem
		if n.isSelf(nodes[0]) {
			assignItem, err = n.tryGetLocalAssign(*item)
			if err != nil {
				log.Error("outbox try get local assignment failed", "pid", pid, "target", target, "itemId", item.Id, "err", err)
				return
			}

		} else {
			assignItem, err = n.tryGetAssign(item.Id, nodes, itemBin)
			if err != nil {
				log.Error("outbox try get assignment failed", "pid", pid, "target", target, "itemId", item.Id, "err", err)
				return
			}
		}

		if err = n.outboxDB.Commit(pid, target, assignItem); err != nil {
			log.Error("outbox commit failed", "pid", pid, "target", target, "err", err)
			return
		}
	}
}

func (n *Node) tryGetLocalAssign(item goarSchema.BundleItem) (assignItem goarSchema.BundleItem, err error) {
	// Use callback function to wait for assignment result
	resultChan := make(chan schema.AssignmentResult, 1)
	closed := make(chan bool, 1)

	// Create temporary assignment handler
	handler := func(result schema.AssignmentResult) {
		if result.Item.Id == item.Id {
			select {
			case <-closed:
				// channel is closed, do nothing
				return
			default:
			}

			resultChan <- result
		}
	}

	// Register handler
	n.AddAssignmentHandler(handler)

	// Ensure cleanup when function ends
	defer func() {
		n.RemoveAssignmentHandler(handler)
		close(closed)
		close(resultChan)
	}()

	for retry := 0; retry < 5; retry++ {
		log.Debug("outbox try get assign retry", "retry", retry)
		if retry > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		err = n.Handle(item)
		if err != nil {
			continue
		}

		// Wait for assignment result
		select {
		case assignmentResult := <-resultChan:
			if assignmentResult.Error != nil {
				err = assignmentResult.Error
				if err == schema.ErrDuplicateItem {
					log.Debug("outbox try get duplicate item, exit", "msgid", item.Id)
					return
				}
				continue
			}
			log.Debug("outbox try get assign success", "msgid", item.Id)
			return
		case <-time.After(2 * time.Second):
			err = fmt.Errorf("timeout waiting for assignment result")
			continue
		}
	}

	return
}

func (n *Node) tryGetAssign(msgId string, nodes []registrySchema.Node, itemBin []byte) (assignItem goarSchema.BundleItem, err error) {
	if len(nodes) == 0 {
		return
	}

	// todo: use all nodes in the future
	cli := n.sdk.Client
	if nodes[0].URL != n.hymxURL {
		cli = sdk.NewClient(nodes[0].URL)
	}

	sleepSchedule := []time.Duration{
		0,
		500 * time.Millisecond,
		1 * time.Second,
		3 * time.Second,
		5 * time.Second,
		10 * time.Second,
		20 * time.Second,
		50 * time.Second,
		80 * time.Second,
	}

	for retry := 0; retry < len(sleepSchedule); retry++ {
		cli.Send(itemBin)

		if sleepSchedule[retry] > 0 {
			time.Sleep(sleepSchedule[retry])
		}
		assignItem, err = cli.GetAssignByMessage(msgId)
		if err != nil {
			continue
		}
		err = goarUtils.VerifyBundleItem(assignItem)
		if err != nil {
			continue
		}
		accid := ""
		_, accid, _, _, err = utils.Decode(assignItem)
		if err != nil {
			continue
		}
		if accid != nodes[0].AccId {
			err = schema.ErrUnauthorizedNode
			continue
		}

		return assignItem, nil
	}

	return
}

func (n *Node) sendingLock(pid, target string) {
	n.outboxLockMu.Lock()
	defer n.outboxLockMu.Unlock()

	n.outboxSendingLock[pid+target] = true
}

func (n *Node) sendingUnlock(pid, target string) {
	n.outboxLockMu.Lock()
	defer n.outboxLockMu.Unlock()

	delete(n.outboxSendingLock, pid+target)
}

func (n *Node) isSending(pid, target string) bool {
	n.outboxLockMu.RLock()
	defer n.outboxLockMu.RUnlock()

	locked, ok := n.outboxSendingLock[pid+target]
	if !ok {
		return false
	}
	return locked
}
