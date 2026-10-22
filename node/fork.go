package node

import (
	"strconv"
	"time"

	"github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/hymatrix/hymx/utils"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
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

func (n *Node) Fork(pid, checkpointID, nodeURL string, mode vmmSchema.ExecMode) error {
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

	go n.runForkProcess(pid, checkpointID, clients, mode)

	return nil
}

func (n *Node) runForkProcess(pid, checkpointID string, clients []sdk.Client, mode vmmSchema.ExecMode) {
	// init nonce to 0
	nonce := int64(0)

	// load checkpoint if exist
	if curNonce, err := n.Restore(checkpointID); err == nil {
		nonce = curNonce + 1
	}

	delay := 10 * time.Second

	n.wg.Add(1)
	defer n.wg.Done()
	scheduler := ""
	for scheduler == "" {
		select {
		case <-n.ctx.Done():
			return
		default:
		}
		spawn, err := clients[0].GetMessageBin(pid)
		if err == nil {
			scheduler, err = forkScheduler(pid, spawn)
		}
		if err != nil {
			log.Error("fork process can not verify spawn", "pid", pid, "err", err)
			select {
			case <-n.ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}
	// fork all message from source
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
			// todo: ensure message retrieval succeeds even when using multiple clients in the future
			msgItem, err := clients[0].GetMessageBinByNonce(pid, nonce)
			if err != nil {
				time.Sleep(delay)
				continue
			}
			assignItem, err := clients[0].GetAssignBinByNonce(pid, nonce)
			if err != nil {
				log.Error("fork process is in progress, can not get assign", "pid", pid, "nonce", nonce, "err", err)
				time.Sleep(delay)
				continue
			}

			assign, err := verifyForkItem(pid, nonce, scheduler, msgItem, assignItem)
			if err != nil {
				log.Error("fork process is in progress, invalid history item", "pid", pid, "nonce", nonce, "err", err)
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

// forkScheduler reads the scheduler from a spawn already verified by the SDK.
func forkScheduler(pid string, spawn goarSchema.BundleItem) (string, error) {
	if spawn.Id != pid || utils.GetType(spawn) != hymxSchema.TypeProcess {
		return "", schema.ErrInvalidForkItem
	}
	proc, err := utils.DecodeItemToProcess(spawn)
	if err != nil {
		return "", err
	}
	if proc.Scheduler == "" {
		return "", schema.ErrInvalidForkItem
	}
	return proc.Scheduler, nil
}

// verifyForkItem binds SDK-verified items to the requested history position.
func verifyForkItem(pid string, nonce int64, scheduler string, msg, item goarSchema.BundleItem) (assign hymxSchema.Assignment, err error) {
	_, signer, _, instance, err := utils.Decode(item)
	if err != nil {
		return
	}
	assign, ok := instance.(hymxSchema.Assignment)
	if !ok || signer != scheduler || assign.Process != pid || assign.Message != msg.Id || assign.Nonce != strconv.FormatInt(nonce, 10) {
		return assign, schema.ErrInvalidForkItem
	}
	switch utils.GetType(msg) {
	case hymxSchema.TypeProcess:
		if nonce != 0 || msg.Id != pid {
			return assign, schema.ErrInvalidForkItem
		}
	case hymxSchema.TypeMessage:
		if msg.Target != pid || nonce == 0 {
			return assign, schema.ErrInvalidForkItem
		}
	default:
		return assign, schema.ErrInvalidForkItem
	}
	return
}
