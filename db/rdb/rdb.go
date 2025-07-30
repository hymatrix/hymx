package rdb

import (
	"context"

	"github.com/hymatrix/hymx/common"
	"github.com/redis/go-redis/v9"
)

var log = common.NewLog("rdb")

type RDB struct {
	rdb *redis.Client
	ctx context.Context
}

func New(redisUrl string) *RDB {
	redisOpt, err := redis.ParseURL(redisUrl)
	if err != nil {
		panic(err)
	}
	redisOpt.PoolSize = 500
	redisOpt.MinIdleConns = 50
	redisOpt.MaxRetries = 3

	return &RDB{
		rdb: redis.NewClient(redisOpt),
		ctx: context.Background(),
	}
}

func (r *RDB) Close() {
	r.rdb.Close()
}
