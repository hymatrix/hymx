package node

import (
	"github.com/hymatrix/hymx/node/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func (n *Node) runMsgChan() {
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():

			return

		case i := <-n.assignMesChan:

			assign, assignItem, err := n.assignment(i.Pid, i.Item)
			n.assignResChan <- schema.AssignmentResult{
				Pid:        i.Pid,
				Item:       i.Item,
				Assign:     assign,
				AssignItem: assignItem,
				Error:      err,
			}
			if err != nil {
				log.Error("assignment failed", "pid", i.Pid, "itemId", i.Item.Id, "err", err)
				continue
			}

			if err := n.applyMessage(i.Pid, i.AccId, i.Item, i.Message, assign, vmmSchema.ExecModeApply, 0); err != nil {
				log.Error("handle item failed", "pid", i.Pid, "itemId", i.Item.Id, "err", err)
			}

		}
	}
}

func (n *Node) runProcChan() {
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():

			return

		case i := <-n.assignProcChan:

			if err := n.applyProcess(i.Pid, i.AccId, i.Item, i.Process, vmmSchema.ExecModeApply, 0); err != nil {
				log.Error("spawn failed", "pid", i.Pid, "itemId", i.Item.Id, "err", err)
				continue
			}

			assign, assignItem, err := n.assignment(i.Pid, i.Item)
			n.assignResChan <- schema.AssignmentResult{
				Pid:        i.Pid,
				Item:       i.Item,
				Assign:     assign,
				AssignItem: assignItem,
				Error:      err,
			}
			if err != nil {
				log.Error("assignment failed", "pid", i.Pid, "itemId", i.Item.Id, "err", err)
			}

		}
	}
}

func (n *Node) runResultChan() {
	defer n.outputWg.Done()
	for result := range n.resultChan {
		// handle result
		n.resultHandlerLockMu.RLock()
		for _, handler := range n.resultHandlers {
			handler(result)
		}
		n.resultHandlerLockMu.RUnlock()

		// apply mode  => save cache and result
		// replay mode => save cache and result
		// dryrun mode  => NOT save cache and result
		if result.Mode == vmmSchema.ExecModeDryRun {
			continue
		}

		// save cache to db
		for k, v := range result.Cache {
			if err := n.db.SaveCache(result.FromProcess, k, v); err != nil {
				log.Error("save cache failed", "pid", result.FromProcess, "k", k, "v", v)
			}
		}

		// save result to db, remove cache in result
		result.Cache = nil
		if err := n.db.SaveResult(result); err != nil {
			log.Error("save result failed", "msgid", result.ItemId, "err", err)
		}

	}
}

func (n *Node) runAssignmentChan() {
	defer n.outputWg.Done()
	for assignmentResult := range n.assignResChan {
		log.Debug("assign chan get notice", "msgid", assignmentResult.Item.Id)

		// handle assignment success
		n.assignResHandlerLockMu.RLock()
		for _, handler := range n.assignResHandlers {
			handler(assignmentResult)
		}
		n.assignResHandlerLockMu.RUnlock()
	}
}

func (n *Node) runOutboxChan() {
	defer n.outputWg.Done()
	for o := range n.outboxChan {
		n.outbox(o)
	}
}
