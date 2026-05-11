package node

import (
	"context"
	"math/big"
	"strconv"
	"sync"

	"github.com/hymatrix/hymx/chainkit"
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/db/cache"
	"github.com/hymatrix/hymx/db/rdb"
	"github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/hymatrix/hymx/vmm"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/panjf2000/ants/v2"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
)

var log = common.NewLog("node")

type Node struct {
	hymxURL string
	info    *schema.Info

	bundler *goar.Bundler
	sdk     *sdk.SDK

	vmm *vmm.Vmm

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	assignMesChan  chan schema.AssignMessage
	assignProcChan chan schema.AssignProcess
	assignResChan  chan schema.AssignmentResult
	resultChan     <-chan vmmSchema.VmmResult

	// handler
	itemHandlers           []schema.ItemHandler
	itemHandlerLockMu      sync.RWMutex
	assignResHandlers      []schema.AssignResHandler
	assignResHandlerLockMu sync.RWMutex
	resultHandlers         []schema.ResultHandler
	resultHandlerLockMu    sync.RWMutex

	outboxChan        <-chan vmmSchema.Outbox
	outboxSendingLock map[string]bool
	outboxLockMu      sync.RWMutex

	db       schema.IDB
	outboxDB schema.IDBOutbox

	chainkit *chainkit.Chainkit

	recoveryTaskPool *ants.Pool
	registrySpawned  chan struct{}
}

func New(
	signer interface{},
	bundler *goar.Bundler,
	redisURL string,
	arweaveURL string,
	hymxURL string,
	nodeInfo *schema.Info,
	chainkit *chainkit.Chainkit,
) *Node {
	outboxChan := make(chan vmmSchema.Outbox, 1000)
	resultChan := make(chan vmmSchema.VmmResult, 1000)
	registryCh := make(chan struct{}, 1)

	taskPool, err := ants.NewPool(1000)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Node{
		hymxURL: hymxURL,
		info:    nodeInfo,

		bundler: bundler,
		sdk:     sdk.NewFromBundler(hymxURL, bundler),

		vmm: vmm.New(nodeInfo, resultChan, outboxChan, registryCh, signer),

		ctx:    ctx,
		cancel: cancel,

		assignMesChan:  make(chan schema.AssignMessage, 1000),
		assignProcChan: make(chan schema.AssignProcess, 10),
		assignResChan:  make(chan schema.AssignmentResult, 1000),
		resultChan:     resultChan,

		itemHandlers:      []schema.ItemHandler{},
		assignResHandlers: []schema.AssignResHandler{},
		resultHandlers:    []schema.ResultHandler{},

		outboxChan:        outboxChan,
		outboxSendingLock: map[string]bool{},

		db:               rdb.New(redisURL),
		outboxDB:         cache.NewOutbox(),
		recoveryTaskPool: taskPool,
		chainkit:         chainkit,
		registrySpawned:  registryCh,
	}
}

func (n *Node) Run(startMode string) {
	if n.chainkit != nil {
		n.chainkit.Run()
		n.AddAssignResHandler(n.chainkit.AssignmentHandler)
	}

	n.vmm.Run()
	go n.runMsgChan()
	go n.runProcChan()
	go n.runResultChan()
	go n.runAssignmentChan()
	go n.runOutboxChan()

	if startMode == schema.StartModeRebuild {
		log.Info("start mode selected", "startMode", startMode)
		go n.runReplay()
	} else {
		log.Info("start mode selected", "startMode", startMode)
		go n.runRecovery()
	}
	n.runJoin()
	if n.info.Node.Role == registrySchema.RoleMain && startMode == schema.StartModeRebuild {
		n.runDefaultFork(vmmSchema.ExecModeReplay)
	} else {
		n.runDefaultFork(vmmSchema.ExecModeDryRun)
	}
}

func (n *Node) Close() {
	log.Info("node is shutting down")
	n.leave()

	n.cancel()
	n.wg.Wait()

	n.recoveryTaskPool.Release()

	n.runCheckpoint()
	n.vmm.Close()

	if n.chainkit != nil {
		n.chainkit.Close()
	}

	log.Info("node has been shut down")
}

func (n *Node) Info() schema.Info {
	if n.vmm.TokenId() != "" {
		n.info.Token = n.vmm.TokenId()
	}
	if n.vmm.RegistryId() != "" {
		n.info.Registry = n.vmm.RegistryId()
	}
	n.info.VmCount = n.vmm.GetVmCount()
	return *n.info
}

func (n *Node) GetMessage(msgid string) (msg *goarSchema.BundleItem, err error) {
	return n.db.GetMessage(msgid)
}

func (n *Node) GetResult(msgid string) (result *vmmSchema.VmmResult, err error) {
	return n.db.GetResult(msgid)
}

func (n *Node) GetResults(pid string, limit int64) (results []vmmSchema.VmmResult, err error) {
	return n.db.GetResults(pid, limit)
}

func (n *Node) GetMessageByNonce(pid string, nonce int64) (msg *goarSchema.BundleItem, err error) {
	return n.db.GetMessageByNonce(pid, nonce)
}

func (n *Node) GetAssignByNonce(pid string, nonce int64) (assign *goarSchema.BundleItem, err error) {
	return n.db.GetAssignByNonce(pid, nonce)
}

func (n *Node) GetAssignByMessage(msgid string) (assign *goarSchema.BundleItem, err error) {
	res, err := n.db.GetResult(msgid)
	if err != nil {
		return
	}
	if res == nil {
		return
	}
	nonce, err := strconv.ParseInt(res.Nonce, 10, 64)
	if err != nil {
		return
	}
	return n.db.GetAssignByNonce(res.FromProcess, nonce)
}

func (n *Node) GetNode(accid string) (*registrySchema.Node, error) {
	return n.vmm.GetNode(accid)
}

func (n *Node) GetNodes() (map[string]registrySchema.Node, error) {
	return n.vmm.GetNodes()
}

func (n *Node) GetProcesses(accid string) ([]string, error) {
	return n.vmm.GetProcesses(accid)
}

func (n *Node) GetNodesByProcess(pid string) ([]registrySchema.Node, error) {
	return n.vmm.GetNodesByProcess(pid)
}

func (n *Node) GetNonce(pid string) (int64, error) {
	return n.db.GetNonce(pid)
}

func (n *Node) BalanceOf(accid string) (*big.Int, error) {
	return n.vmm.BalanceOf(accid)
}

func (n *Node) StakeOf(accid string) (*big.Int, error) {
	return n.vmm.StakeOf(accid)
}

func (n *Node) GetCache(pid, key string) (string, error) {
	return n.db.GetCache(pid, key)
}

func (n *Node) GetModuleNames() []string {
	return n.vmm.GetModuleNames()
}

func (n *Node) Mount(moduleFormat string, spawner vmmSchema.VmSpawnFunc) error {
	return n.vmm.Mount(moduleFormat, spawner)
}

func (n *Node) AddItemHandler(handler ...schema.ItemHandler) {
	n.itemHandlerLockMu.Lock()
	defer n.itemHandlerLockMu.Unlock()

	n.itemHandlers = append(n.itemHandlers, handler...)
}

func (n *Node) AddResultHandler(handler ...schema.ResultHandler) {
	n.resultHandlerLockMu.Lock()
	defer n.resultHandlerLockMu.Unlock()

	n.resultHandlers = append(n.resultHandlers, handler...)
}

func (n *Node) AddAssignResHandler(handler ...schema.AssignResHandler) {
	n.assignResHandlerLockMu.Lock()
	defer n.assignResHandlerLockMu.Unlock()

	n.assignResHandlers = append(n.assignResHandlers, handler...)
}

func (n *Node) RemoveAssignResHandler(handler schema.AssignResHandler) {
	n.assignResHandlerLockMu.Lock()
	defer n.assignResHandlerLockMu.Unlock()

	for i, h := range n.assignResHandlers {
		if &h == &handler {
			n.assignResHandlers = append(n.assignResHandlers[:i], n.assignResHandlers[i+1:]...)
			break
		}
	}
}

func (n *Node) IsRedirect(pid string) (ok bool, nodes []registrySchema.Node, err error) {
	ok = true
	nodes, err = n.vmm.GetNodesByProcess(pid)
	if err != nil {
		return
	}

	for _, node := range nodes {
		if node.AccId == n.bundler.Address {
			ok = false
			return
		}
	}
	return
}

func (n *Node) isSelf(node registrySchema.Node) bool {
	if n.Info().Node.AccId == node.AccId {
		if n.Info().Node.URL == node.URL {
			return true
		}
	}
	return false
}

func (n *Node) waitRegistrySpawned() {
	if n.vmm.RegistryId() != "" {
		return
	}
	// wait for registry spawned
	<-n.registrySpawned
}
