package node

import (
	"time"

	"github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/hymatrix/hymx/utils"
)

func (n *Node) runDefaultFork() {
	// fork registry
	if n.info.Registry != "" {
		ckpId, err := n.db.GetCheckpointIndex(n.info.Registry)
		if err != nil {
			log.Warn("fork registry can not get checkpoint index", "pid", n.info.Registry, "err", err)
		}

		if err := n.Fork(n.info.Registry, ckpId, n.hymxURL); err != nil {
			log.Error("fork registry failed", "registry", n.info.Registry, "url", n.hymxURL, "err", err)
		}
	}
	// fork token
	if n.info.Token != "" {
		ckpId, err := n.db.GetCheckpointIndex(n.info.Token)
		if err != nil {
			log.Warn("fork core token can not get checkpoint index", "pid", n.info.Registry, "err", err)
		}

		if err := n.Fork(n.info.Token, ckpId, n.hymxURL); err != nil {
			log.Error("fork core token failed", "token", n.info.Token, "url", n.hymxURL, "err", err)
		}
	}
}

func (n *Node) Fork(pid, checkpointID, nodeURL string) error {
	if n.vmm.IsExists(pid) {
		log.Error("fork process is already exists", "pid", pid)
		return schema.ErrProcessAlreadyExists
	}

	clients := []sdk.Client{}
	if nodeURL != "" {
		clients = append(clients, *sdk.NewClient(nodeURL))
	} else {
		nodes, err := n.vmm.GetNodesByProcess(pid)
		if err != nil {
			log.Error("fork process not found nodes", "err", err)
			return err
		}
		if len(nodes) == 0 {
			log.Error("fork process not found nodes", "pid", pid)
			return schema.ErrNotFoundNodes
		}
		clients = make([]sdk.Client, len(nodes))
		for _, node := range nodes {
			clients = append(clients, *sdk.NewClient(node.URL))
		}
	}

	go n.runForkProcess(pid, checkpointID, clients)

	return nil
}

func (n *Node) runForkProcess(pid, checkpointID string, clients []sdk.Client) {
	// init nonce to 0
	nonce := int64(0)

	// load checkpoint if exist
	if curNonce, err := n.Restore(checkpointID); err == nil {
		nonce = curNonce + 1
	}

	delay := 10 * time.Second

	n.wg.Add(1)
	defer n.wg.Done()
	// fork all message from source
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
			// todo: ensure message retrieval succeeds even when using multiple clients in the future
			msgItem, err := clients[0].GetMessageByNonce(pid, nonce)
			if err != nil {
				time.Sleep(delay)
				continue
			}
			assignItem, err := clients[0].GetAssignByNonce(pid, nonce)
			if err != nil {
				log.Error("fork process is in progress, can not get assign", "pid", pid, "nonce", nonce, "err", err)
				time.Sleep(delay)
				continue
			}

			assign, err := utils.TagsToAssignment(assignItem.Tags)
			if err != nil {
				log.Error("fork process is in progress, decode assign failed", "pid", pid, "nonce", nonce, "err", err)
				time.Sleep(delay)
				continue
			}

			if err = n.HandleDryRun(msgItem, assign, -1); err != nil {
				log.Error("fork process is in progress, dry run failed", "pid", pid, "nonce", nonce, "err", err)
				time.Sleep(delay)
				continue
			}

			nonce++
		}

	}
}
