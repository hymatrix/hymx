package node

import (
	"errors"
	"net/url"
	"sort"
	"time"

	"github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/sdk"
	sdkSchema "github.com/hymatrix/hymx/sdk/schema"
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
		clients = make([]sdk.Client, 0, len(nodes))
		seen := map[string]bool{}
		for _, node := range nodes {
			if node.AccId != n.info.Node.AccId && node.URL != "" && !seen[node.URL] {
				clients = append(clients, *sdk.NewClient(node.URL))
				seen[node.URL] = true
			}
		}
		if len(clients) == 0 {
			log.Error("fork process not found remote nodes", "pid", pid)
			return schema.ErrNotFoundNodes
		}
		sort.Slice(clients, func(i, j int) bool { return clients[i].BaseURL() < clients[j].BaseURL() })
	}

	go n.runForkProcess(pid, checkpointID, clients, nodeURL, mode)

	return nil
}

func (n *Node) runForkProcess(pid, checkpointID string, clients []sdk.Client, bootstrapURL string, mode vmmSchema.ExecMode) {
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

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
			msgItem, err := clients[0].GetMessageByNonce(pid, nonce)
			if err != nil {
				clients = n.retryForkClients(pid, clients, bootstrapURL, err)
				time.Sleep(delay)
				continue
			}
			assignItem, err := clients[0].GetAssignByNonce(pid, nonce)
			if err != nil {
				log.Error("fork process is in progress, can not get assign", "pid", pid, "nonce", nonce, "err", err)
				clients = n.retryForkClients(pid, clients, bootstrapURL, err)
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

func (n *Node) retryForkClients(pid string, clients []sdk.Client, bootstrapURL string, err error) []sdk.Client {
	var requestErr *url.Error
	retry := errors.As(err, &requestErr)
	var status *sdkSchema.HTTPStatusError
	if errors.As(err, &status) {
		retry = status.StatusCode/100 == 5
	}

	failedURL := clients[0].BaseURL()
	// Refresh on every failure: a stale URL can answer 404 for a process it no longer hosts.
	clients = n.refreshForkClients(pid, clients, bootstrapURL)
	if retry && len(clients) > 1 && clients[0].BaseURL() == failedURL {
		// Try the next candidate on the next polling cycle, retaining fallback URLs.
		failed := clients[0]
		copy(clients, clients[1:])
		clients[len(clients)-1] = failed
	}
	if clients[0].BaseURL() != failedURL {
		log.Info("fork source changed", "pid", pid, "oldURL", failedURL, "newURL", clients[0].BaseURL())
	}
	return clients
}

func (n *Node) refreshForkClients(pid string, clients []sdk.Client, bootstrapURL string) []sdk.Client {
	nodes, err := n.vmm.GetNodesByProcess(pid)
	if err != nil {
		return clients
	}
	urls := map[string]bool{}
	for _, node := range nodes {
		if node.AccId == n.info.Node.AccId || node.URL == "" {
			continue
		}
		urls[node.URL] = true
	}
	if len(urls) == 0 {
		return clients
	}
	bootstrapFallback := bootstrapURL != "" && !urls[bootstrapURL] &&
		len(clients) > 0 && clients[0].BaseURL() == bootstrapURL
	if bootstrapURL != "" {
		urls[bootstrapURL] = true
	}
	refreshed := make([]sdk.Client, 0, len(urls))
	changed := false
	for _, client := range clients {
		if urls[client.BaseURL()] {
			refreshed = append(refreshed, client)
			delete(urls, client.BaseURL())
		} else {
			client.Close()
			changed = true
			log.Info("fork candidate removed", "pid", pid, "url", client.BaseURL())
		}
	}
	added := make([]string, 0, len(urls))
	for url := range urls {
		added = append(added, url)
	}
	sort.Strings(added)
	for _, url := range added {
		refreshed = append(refreshed, *sdk.NewClient(url))
		log.Info("fork candidate added", "pid", pid, "url", url)
	}
	// Demote a failed, unlisted configured source without reordering other fallbacks.
	if bootstrapFallback {
		for i, client := range refreshed {
			if client.BaseURL() == bootstrapURL && i < len(refreshed)-1 {
				copy(refreshed[i:], refreshed[i+1:])
				refreshed[len(refreshed)-1] = client
				changed = true
				break
			}
		}
	}
	if !changed && len(added) == 0 {
		return clients
	}
	return refreshed
}
