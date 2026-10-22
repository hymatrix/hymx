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
	goarUtils "github.com/permadao/goar/utils"
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

	// The scheduler comes from the spawn item, which is bound to pid by its id and signature.
	// Every forked assignment must be signed by this scheduler.
	scheduler := ""
	for scheduler == "" {
		select {
		case <-n.ctx.Done():
			return
		default:
		}
		spawn, err := clients[0].GetMessage(pid)
		if err == nil {
			scheduler, err = forkScheduler(pid, spawn)
		}
		if err != nil {
			log.Error("fork process can not verify spawn", "pid", pid, "err", err)
			time.Sleep(delay)
		}
	}

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

// forkScheduler returns the scheduler of pid from its spawn item.
// The spawn item must be validly signed and its id must equal pid.
func forkScheduler(pid string, spawn goarSchema.BundleItem) (scheduler string, err error) {
	if spawn.Id != pid {
		return "", schema.ErrInvalidForkItem
	}
	if err = goarUtils.VerifyBundleItem(spawn); err != nil {
		return
	}
	_, _, _, instance, err := utils.Decode(spawn)
	if err != nil {
		return
	}
	proc, ok := instance.(hymxSchema.Process)
	if !ok || proc.Scheduler == "" {
		return "", schema.ErrInvalidForkItem
	}
	return proc.Scheduler, nil
}

// verifyForkItem checks one forked history entry: both items are validly signed,
// and the assignment is signed by the scheduler and binds msgItem to (pid, nonce).
func verifyForkItem(pid string, nonce int64, scheduler string, msgItem, assignItem goarSchema.BundleItem) (assign hymxSchema.Assignment, err error) {
	if err = goarUtils.VerifyBundleItem(msgItem); err != nil {
		return
	}
	if err = goarUtils.VerifyBundleItem(assignItem); err != nil {
		return
	}
	_, signer, _, instance, err := utils.Decode(assignItem)
	if err != nil {
		return
	}
	assign, ok := instance.(hymxSchema.Assignment)
	if !ok || signer != scheduler || assign.Process != pid || assign.Message != msgItem.Id ||
		assign.Nonce != strconv.FormatInt(nonce, 10) {
		return assign, schema.ErrInvalidForkItem
	}
	return
}
