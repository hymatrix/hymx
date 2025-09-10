package chainkit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hymatrix/hymx/chainkit/schema"
	"github.com/hymatrix/hymx/common"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/redis/go-redis/v9"
)

var log = common.NewLog("node")

type Chainkit struct {
	node              schema.INodeDB
	operator          schema.IOperator
	aggregationPolicy schema.AggregationPolicy

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	redis  *redis.Client
}

func New(op schema.IOperator, node schema.INodeDB, redisUrl string) *Chainkit {
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
		ctx:    ctx,
		cancel: cancel,
		redis:  redis.NewClient(redisOpt),
	}
}

func (c *Chainkit) Run() {
	log.Info("chainkit run")

	go c.tryByTime()
	go c.check()
}

func (c *Chainkit) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	log.Info("chainkit closed")
}

// 上传一笔 BundleItem 交易，这个函数不会真正上传一笔交易，只是将交易放到上传队列中
// 等待交易打包后才上传
func (c *Chainkit) Upload(tx goarSchema.BundleItem) error {
	if tx.Id == "" {
		return errors.New("invalid bundle item: empty id")
	}

	// 使用 Redis Set 去重并记录待上传的 txid
	if err := c.addToUploads(tx.Id); err != nil {
		return err
	}

	c.tryByCount()
	return nil
}

// 下载一笔交易
// 1. Qraphql 查询父交易
// 2. 通过父交易下载子交易
// 3. 验证子交易
// 4. 返回交易
func (c *Chainkit) DownloadByTxid(txid string) (goarSchema.BundleItem, error) {
	parentTxID, err := c.getParentTxid(txid)
	if err != nil {
		return goarSchema.BundleItem{}, err
	}
	items, err := c.download(parentTxID, []string{txid})
	if err != nil {
		return goarSchema.BundleItem{}, err
	}
	if len(items) == 0 {
		return goarSchema.BundleItem{}, errors.New("download failed")
	}
	return *items[0], nil
}

// 下载一个 process 的所有交易, 从指定的 Nonce 开始到最新交易
// todo: begin and end nonce
// signer 参数（拿 txid=pid 的交易）
func (c *Chainkit) DownloadByPid(pid string, nonce int64) ([]goarSchema.BundleItem, error) {
	panic("unimplemented")
}

// 执行一个 GraphQL 查询
func (c *Chainkit) Query(query string) ([]byte, error) {
	return c.operator.GraphQL(query)
}
