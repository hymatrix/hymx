package chainkit

import (
	"context"
	"errors"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/hymatrix/hymx/chainkit/schema"
	"github.com/hymatrix/hymx/common"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/redis/go-redis/v9"
)

var log = common.NewLog("node")

type Chainkit struct {
	node              schema.INode
	operator          schema.IOperator
	aggregationPolicy schema.AggregationPolicy
	scheduler         *gocron.Scheduler

	ctx    context.Context
	cancel context.CancelFunc
	redis  *redis.Client
}

func New(op schema.IOperator, node schema.INode, redisUrl string) *Chainkit {
	ctx, cancel := context.WithCancel(context.Background())

	redisOpt, err := redis.ParseURL(redisUrl)
	if err != nil {
		panic(err)
	}
	redisOpt.PoolSize = 500
	redisOpt.MinIdleConns = 50
	redisOpt.MaxRetries = 3

	return &Chainkit{
		node:     node,
		operator: op,
		aggregationPolicy: schema.AggregationPolicy{
			MaxItems: 1000,
			MaxDelay: 5 * time.Minute,
		},
		scheduler: gocron.NewScheduler(time.UTC),
		ctx:       ctx,
		cancel:    cancel,
		redis:     redis.NewClient(redisOpt),
	}
}

func (c *Chainkit) Run() {
	log.Info("chainkit run")
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

// Upload a BundleItem transaction. This function doesn't actually upload the transaction,
// but adds it to the upload queue and waits for batch processing before uploading
func (c *Chainkit) Upload(tx goarSchema.BundleItem) error {
	if tx.Id == "" {
		return errors.New("invalid bundle item: empty id")
	}

	// Use Redis Set to deduplicate and record pending upload txids
	return c.addPendingTx(tx.Id)
}

// Download a transaction
func (c *Chainkit) DownloadByTxid(txid string) (*goarSchema.BundleItem, error) {
	return c.operator.Download(txid)
}

// Download all transactions of a process, from specified Nonce to latest transaction
// todo: begin and end nonce
// signer parameter (get transaction with txid=pid)
func (c *Chainkit) DownloadByPid(pid string, beginNonce, endNonce int64) (results []*schema.DownloadResult, err error) {
	// 1. 下载 txid = pid 的交易，这笔交易是 nonce=0 的 spawn 交易，用于创建进程
	spawnMsg, err := c.operator.Download(pid)
	if err != nil {
		log.Error("DownloadByPid failed", "pid", pid, "err", err)
		return nil, err
	}
	if spawnMsg == nil {
		log.Error("DownloadByPid failed, spawnMsg is nil", "pid", pid)
		return nil, schema.ErrSpawnTxNotFound
	}
	// 2. 校验交易，获取 signer
	if err = c.verifySpawnMsg(spawnMsg); err != nil {
		log.Error("verifySpawnMsg failed", "pid", pid, "err", err)
		return nil, err
	}
	// 3. 下载所有 [beginNonce, endNonce] 范围内的交易
	items, err := c.downloadByNonce("", pid, beginNonce, endNonce)
	if err != nil {
		log.Error("downloadByNonce failed", "pid", pid, "err", err)
		return nil, err
	}
	return items, nil
}

// Execute a GraphQL query
func (c *Chainkit) Query(query string) ([]byte, error) {
	return c.operator.GraphQL(query)
}
