package rdb

import (
	"encoding/json"
	"fmt"

	"github.com/hymatrix/hymx/db/rdb/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/redis/go-redis/v9"
)

func (r *RDB) PushOutbox(pid, target string, message goarSchema.BundleItem) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return r.rdb.RPush(r.ctx, schema.RdbOutboxPrefix+pid+":"+target, data).Err()
}

func (r *RDB) PeekOutbox(pid, target string) (*goarSchema.BundleItem, error) {
	data, err := r.rdb.LIndex(r.ctx, schema.RdbOutboxPrefix+pid+":"+target, 0).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	message := &goarSchema.BundleItem{}
	if err = json.Unmarshal(data, message); err != nil {
		return nil, err
	}
	return message, nil
}

func (r *RDB) CommitOutbox(pid, target string) error {
	err := r.rdb.LPop(r.ctx, schema.RdbOutboxPrefix+pid+":"+target).Err()
	if err == redis.Nil {
		return fmt.Errorf("no pending target sequence for %s/%s", pid, target)
	}
	return err
}
