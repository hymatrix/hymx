package chainkit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/hymatrix/hymx/chainkit/optgoar"
	"github.com/hymatrix/hymx/chainkit/schema"
	"github.com/hymatrix/hymx/common"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/redis/go-redis/v9"
)

var log = common.NewLog("chainkit")

type Chainkit struct {
	node      schema.INode
	operator  schema.IOperator
	scheduler *gocron.Scheduler

	ctx    context.Context
	cancel context.CancelFunc
	redis  *redis.Client
	mu     sync.Mutex // Mutex to prevent concurrent execution
}

func New(node schema.INode, config schema.Config) *Chainkit {
	ctx, cancel := context.WithCancel(context.Background())
	var op schema.IOperator
	if config.OptType == "goar" {
		wallet, err := goar.NewWalletFromPath(config.Keyfile, "https://arweave.net")
		if err != nil {
			panic(err)
		}
		op = optgoar.New(wallet, ctx)
	} else {
		panic("unsupported opt type")
	}

	redisOpt, err := redis.ParseURL(config.RedisUrl)
	if err != nil {
		panic(err)
	}
	redisOpt.PoolSize = 500
	redisOpt.MinIdleConns = 50
	redisOpt.MaxRetries = 3

	return &Chainkit{
		node:      node,
		operator:  op,
		scheduler: gocron.NewScheduler(time.UTC),
		ctx:       ctx,
		cancel:    cancel,
		redis:     redis.NewClient(redisOpt),
	}
}

func (c *Chainkit) Run() {
	log.Info("chainkit running")
	c.scheduler.StartAsync()
	c.runJobs()
}

func (c *Chainkit) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.scheduler != nil {
		c.scheduler.Stop()
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

	// Use Redis Set to deduplicate and record pending upload txids
	return c.addPending(tx.Id)
}

// Download a transaction
func (c *Chainkit) DownloadByTxid(txid string) (*goarSchema.BundleItem, error) {
	return c.operator.Download(txid)
}

// Download all transactions of a process, from specified Nonce to latest transaction
func (c *Chainkit) DownloadByPid(pid string, beginNonce, endNonce int64) (results []*schema.DownloadResult, err error) {
	return nil, nil
}

// Execute a GraphQL query
func (c *Chainkit) Query(query string) ([]byte, error) {
	return c.operator.GraphQL(query)
}
