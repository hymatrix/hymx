package node

import (
	"time"

	"github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/hymatrix/hymx/utils"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func (n *Node) runDefaultFork(mode vmmSchema.ExecMode) {
	// fork registry
	if n.info.Registry != "" {
		ckpId, err := n.db.GetCheckpointIndex(n.info.Registry)
		if err != nil {
			log.Warn("fork registry can not get checkpoint index", "pid", n.info.Registry, "err", err)
		}

		if err := n.Fork(n.info.Registry, ckpId, n.hymxURL, mode); err != nil {
			log.Error("fork registry failed", "registry", n.info.Registry, "url", n.hymxURL, "err", err)
		}
	}
	// fork token
	if n.info.Token != "" {
		ckpId, err := n.db.GetCheckpointIndex(n.info.Token)
		if err != nil {
			log.Warn("fork core token can not get checkpoint index", "pid", n.info.Registry, "err", err)
		}

		if err := n.Fork(n.info.Token, ckpId, n.hymxURL, mode); err != nil {
			log.Error("fork core token failed", "token", n.info.Token, "url", n.hymxURL, "err", err)
		}
	}
}

// Fork uses a fixed nodeURL when provided, or follows Registry URLs when it is empty.
func (n *Node) Fork(pid, checkpointID, nodeURL string, mode vmmSchema.ExecMode) error {
	if n.vmm.IsExists(pid) {
		log.Error("fork process is already exists", "pid", pid)
		return schema.ErrProcessAlreadyExists
	}

	discover := nodeURL == ""
	var client *sdk.Client
	if discover {
		var err error
		nodeURL, client, err = n.refreshForkClient(pid, "", nil)
		if err != nil {
			return err
		}
		if client == nil {
			return schema.ErrNotFoundNodes
		}
	} else {
		client = sdk.NewClient(nodeURL)
	}

	n.wg.Add(1)
	go n.runForkProcess(pid, checkpointID, discover, nodeURL, client, mode)

	return nil
}

func (n *Node) refreshForkClient(pid, nodeURL string, client *sdk.Client) (string, *sdk.Client, error) {
	nodes, err := n.vmm.GetNodesByProcess(pid)
	if err != nil {
		return nodeURL, client, err
	}
	if len(nodes) == 0 || nodes[0].URL == "" || nodes[0].AccId == n.info.Node.AccId {
		return nodeURL, client, nil
	}
	if nodes[0].URL != nodeURL {
		if client != nil {
			client.Close()
		}
		nodeURL = nodes[0].URL
		client = sdk.NewClient(nodeURL)
	}
	return nodeURL, client, nil
}

func (n *Node) runForkProcess(pid, checkpointID string, discover bool, nodeURL string, client *sdk.Client, mode vmmSchema.ExecMode) {
	defer n.wg.Done()
	defer func() { client.Close() }()
	// init nonce to 0
	nonce := int64(0)

	// load checkpoint if exist
	if curNonce, err := n.Restore(checkpointID); err == nil {
		nonce = curNonce + 1
	}

	delay := 10 * time.Second

	// fork all message from source
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
			msgItem, err := client.GetMessageByNonce(pid, nonce)
			if err != nil {
				if discover {
					nodeURL, client, _ = n.refreshForkClient(pid, nodeURL, client)
				}
				time.Sleep(delay)
				continue
			}
			assignItem, err := client.GetAssignByNonce(pid, nonce)
			if err != nil {
				log.Error("fork process is in progress, can not get assign", "pid", pid, "nonce", nonce, "err", err)
				if discover {
					nodeURL, client, _ = n.refreshForkClient(pid, nodeURL, client)
				}
				time.Sleep(delay)
				continue
			}

			assign, err := utils.TagsToAssignment(assignItem.Tags)
			if err != nil {
				log.Error("fork process is in progress, decode assign failed", "pid", pid, "nonce", nonce, "err", err)
				time.Sleep(delay)
				continue
			}

			if err = n.HandleMode(msgItem, assign, mode, -1); err != nil {
				log.Error("fork process is in progress, handle mode failed", "pid", pid, "nonce", nonce, "err", err)
				time.Sleep(delay)
				continue
			}

			nonce++
		}

	}
}
