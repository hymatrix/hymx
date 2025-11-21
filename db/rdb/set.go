package rdb

import (
	"encoding/json"
	"fmt"

	"github.com/hymatrix/hymx/db/rdb/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/redis/go-redis/v9"
)

func (r *RDB) IsExist(pid string) (ok bool, err error) {
	_, err = r.rdb.Get(r.ctx, schema.RdbProcessNoncePrefix+pid).Bytes()
	if err != nil {
		if err != redis.Nil {
			log.Error("is exist error", "err", err)
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (r *RDB) Commit(pid string, nonce int64, msg, assign goarSchema.BundleItem) (err error) {
	msgB, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	assignB, err := json.Marshal(assign)
	if err != nil {
		return err
	}

	// Use Pipeline to ensure atomicity
	pipe := r.rdb.Pipeline()
	// set nonce, never expir
	pipe.Set(r.ctx, schema.RdbProcessNoncePrefix+pid, fmt.Sprintf("%d", nonce), 0)
	// push msg & assign
	pipe.RPush(r.ctx, schema.RdbProcessMsgsPrefix+pid, string(msgB))
	pipe.RPush(r.ctx, schema.RdbProcessAssignmentPrefix+pid, string(assignB))
	// set index: msgid to pid+{{nonce}}
	pipe.HSet(r.ctx, schema.RdbMsgIndex, msg.Id, pid+":"+fmt.Sprintf("%d", nonce))
	_, err = pipe.Exec(r.ctx)
	return
}

func (r *RDB) SaveResult(result vmmSchema.VmmResult) (err error) {
	pipe := r.rdb.Pipeline()

	// save result
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}
	pipe.HSet(r.ctx, schema.RdbMsgResult, result.ItemId, resultBytes)
	// save result to process results
	// processid -> msgid
	pipe.RPush(r.ctx, schema.RdbMsgResultsPrefix+result.FromProcess, result.ItemId)

	// Pipeline
	_, err = pipe.Exec(r.ctx)
	return
}

func (r *RDB) RevertSpawn(pid string) error {
	pipe := r.rdb.Pipeline()
	pipe.Del(r.ctx, schema.RdbProcessNoncePrefix+pid)
	pipe.Del(r.ctx, schema.RdbProcessMsgsPrefix+pid)
	pipe.Del(r.ctx, schema.RdbProcessAssignmentPrefix+pid)
	pipe.HDel(r.ctx, schema.RdbMsgIndex, pid)
	_, err := pipe.Exec(r.ctx)
	return err
}

func (r *RDB) SaveCheckpointIndex(pid, id string) error {
	key := schema.RdbCheckpointIndexPrefix + pid
	return r.rdb.Set(r.ctx, key, id, 0).Err()
}

func (r *RDB) SaveCache(pid, key, value string) error {
	return r.rdb.Set(r.ctx, schema.RdbCachePrefix+pid+":"+key, value, 0).Err()
}
