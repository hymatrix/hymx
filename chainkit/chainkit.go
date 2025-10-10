package chainkit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/hymatrix/hymx/chainkit/optgoar"
	"github.com/hymatrix/hymx/chainkit/schema"
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/db/rdb"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

var log = common.NewLog("chainkit")

type Chainkit struct {
	db       schema.IDB       // chainkit local db
	nodeDB   schema.INodeDB   // node db, readonly functions
	operator schema.IOperator // interfaces with different blockchains

	scheduler gocron.Scheduler

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex // Mutex to prevent concurrent execution
}

func New(config schema.Config) *Chainkit {
	ctx, cancel := context.WithCancel(context.Background())
	var op schema.IOperator
	switch config.OptType {
	case "goar":
		wallet, err := goar.NewWalletFromPath(config.Keyfile, "https://arweave.net")
		if err != nil {
			panic(err)
		}
		op = optgoar.New(wallet, ctx)
	default:
		panic("unsupported opt type")
	}

	scheduler, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
	if err != nil {
		panic(err)
	}

	return &Chainkit{
		nodeDB:    rdb.New(config.NodeRedisUrl),
		db:        rdb.NewChainkitDB(config.RedisUrl),
		operator:  op,
		scheduler: scheduler,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (c *Chainkit) Run() {
	c.scheduler.Start()
	c.runJobs()
	log.Info("chainkit running")
}

func (c *Chainkit) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.scheduler != nil {
		c.scheduler.Shutdown()
	}
	log.Info("chainkit closed")
}

func (c *Chainkit) AssignmentHandler(assignmentResult nodeSchema.AssignmentResult) {
	log.Debug("call assignment handler", "msgid", assignmentResult.Item.Id)
	c.Upload(assignmentResult.Item)
}

// Upload a BundleItem transaction. This function doesn't actually upload the transaction,
// but adds it to the upload queue and waits for batch processing before uploading
func (c *Chainkit) Upload(tx goarSchema.BundleItem) error {
	if tx.Id == "" {
		return errors.New("invalid bundle item: empty id")
	}

	// Check if transaction is already uploaded
	uploaded, err := c.db.IsUploaded(tx.Id)
	if err != nil {
		log.Error("IsUploaded failed", "txid", tx.Id, "err", err)
		return err
	}
	if uploaded {
		log.Debug("txid already uploaded", "txid", tx.Id)
		return nil
	}

	// Use Redis Set to deduplicate and record pending upload txids
	return c.db.AddPending(tx.Id)
}

// Download a transaction
func (c *Chainkit) DownloadByTxid(txid string) (*goarSchema.BundleItem, error) {
	return c.downloadByTxid(txid)
}

// Download all transactions of a process, from specified Nonce to latest transaction
func (c *Chainkit) DownloadByPid(pid string, beginNonce, endNonce int64) (results []*schema.DownloadResult, err error) {
	log.Debug("DownloadByPid", "pid", pid, "beginNonce", beginNonce, "endNonce", endNonce)
	// 1. download spawn transaction, txid = pid, nonce=0
	log.Debug("download spawn transaction", "pid", pid)
	spawnItem, err := c.downloadByTxid(pid)
	if err != nil {
		log.Error("DownloadByPid failed", "pid", pid, "err", err)
		return nil, err
	}
	if spawnItem == nil {
		log.Error("DownloadByPid failed, spawnMsg is nil", "pid", pid)
		return nil, schema.ErrSpawnTxNotFound
	}
	log.Debug("verify spawn transaction success", "pid", pid, "txid", spawnItem.Id)
	// 2. verify spawn message
	if err = c.verifyMessage(spawnItem, hymxSchema.TypeProcess); err != nil {
		log.Error("verifyProcessMsg failed", "pid", pid, "err", err)
		return nil, err
	}

	// 3. download transactions range [beginNonce, endNonce]
	arAddressOwner, err := goarUtils.OwnerToAddress(spawnItem.Owner)
	if err != nil {
		log.Error("OwnerToAddress failed", "pid", pid, "err", err)
		return nil, err
	}
	log.Debug("OwnerToAddress ", "owner", arAddressOwner)
	items, err := c.downloadByNonce(arAddressOwner, pid, beginNonce, endNonce)
	if err != nil {
		log.Error("downloadByNonce failed", "pid", pid, "err", err)
		return nil, err
	}
	log.Debug("items count", "count", len(items))

	return items, nil
}

// Execute a GraphQL query
func (c *Chainkit) Query(query string) ([]byte, error) {
	return c.operator.GraphQL(query)
}
