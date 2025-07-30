package node

import (
	"fmt"
	"time"
)

func (n *Node) runMsgChan() {
	n.wg.Add(1)
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():

			return

		case i := <-n.assignMesChan:

			assign, _, err := n.assignment(i.Pid, i.Item)
			if err != nil {
				log.Error("assignment failed", "pid", i.Pid, "itemId", i.Item.Id, "err", err)
				continue
			}

			if err := n.handleMessage(i.Pid, i.AccId, i.Item, i.Message, assign, false, 0); err != nil {
				log.Error("handle item failed", "pid", i.Pid, "itemId", i.Item.Id, "err", err)
			}

		}
	}
}

func (n *Node) runProcChan() {
	n.wg.Add(1)
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():

			return

		case i := <-n.assignProcChan:

			if err := n.handleProcess(i.Pid, i.AccId, i.Item, i.Process, false, 0); err != nil {
				log.Error("spawn failed", "pid", i.Pid, "itemId", i.Item.Id, "err", err)
				continue
			}

			if _, _, err := n.assignment(i.Pid, i.Item); err != nil {
				log.Error("assignment failed", "pid", i.Pid, "itemId", i.Item.Id, "err", err)
			}

		}
	}
}

func (n *Node) runResultChan() {
	n.wg.Add(1)
	defer n.wg.Done()
	for {
		select {
		case <-n.ctx.Done():

			return

		case result := <-n.resultChan:
			// handle result
			n.resultHandlerLockMu.RLock()
			for _, handler := range n.resultHandlers {
				handler(result)
			}
			n.resultHandlerLockMu.RUnlock()

			// save cache to db
			for k, v := range result.Cache {
				if err := n.db.SaveCache(result.FromProcess, k, v); err != nil {
					log.Error("save cache failed", "pid", result.FromProcess, "k", k, "v", v)
				}
			}

			// save result to db, remove cache in result
			result.Cache = nil
			result.Timestamp = fmt.Sprintf("%d", time.Now().UnixMilli())
			if err := n.db.SaveResult(result); err != nil {
				log.Error("save result failed", "msgid", result.ItemId, "err", err)
			}

		}
	}
}

func (n *Node) runOutboxChan() {
	n.wg.Add(1)
	defer n.wg.Done()
	for {
		select {

		case <-n.ctx.Done():

			return

		case o := <-n.outboxChan:

			n.outbox(o)

		}
	}
}
