package rdb

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/hymatrix/hymx/db/rdb/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/redis/go-redis/v9"
)

func (r *RDB) GetNonce(pid string) (nonce int64, err error) {
	exist, err := r.IsExist(pid)
	if err != nil {
		return 0, err
	}
	if !exist {
		return -1, nil
	}

	key := schema.RdbProcessNoncePrefix + pid
	val, err := r.rdb.Get(r.ctx, key).Result()
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(val, 10, 64)
}

func (r *RDB) GetMessage(msgid string) (msg *goarSchema.BundleItem, err error) {
	res, err := r.rdb.HGet(r.ctx, schema.RdbMsgIndex, msgid).Result()
	if err != nil {
		if err == redis.Nil {
			log.Warn("can not get msg", "msgid", msgid, "err", err)
			err = nil
		}
		return
	}
	pid, nonce := strings.Split(res, ":")[0], strings.Split(res, ":")[1]

	// Convert nonce string to int64
	nonceInt, err := strconv.ParseInt(nonce, 10, 64)
	if err != nil {
		return
	}

	return r.GetMessageByNonce(pid, nonceInt)
}

func (r *RDB) GetMessageByNonce(pid string, nonce int64) (msg *goarSchema.BundleItem, err error) {
	// Get message from list by index
	msgStr, err := r.rdb.LIndex(r.ctx, schema.RdbProcessMsgsPrefix+pid, nonce).Result()
	if err != nil {
		if err == redis.Nil {
			log.Warn("can not get msg by nonce", "pid", pid, "nonce", nonce, "err", err)
			err = nil
		}
		return
	}
	// Unmarshal message
	msg = &goarSchema.BundleItem{}
	err = json.Unmarshal([]byte(msgStr), &msg)
	return
}

func (r *RDB) GetAssignByNonce(pid string, nonce int64) (assign *goarSchema.BundleItem, err error) {
	// Get assign from list by index
	assignStr, err := r.rdb.LIndex(r.ctx, schema.RdbProcessAssignmentPrefix+pid, nonce).Result()
	if err != nil {
		if err == redis.Nil {
			log.Warn("can not get assign", "pid", pid, "nonce", nonce, "err", err)
			err = nil
		}
		return
	}

	// Unmarshal assign
	assign = &goarSchema.BundleItem{}
	err = json.Unmarshal([]byte(assignStr), assign)
	return
}

func (r *RDB) GetAllProcess() (processIds []string, curNonces []int64, err error) {
	processIds = make([]string, 0)
	curNonces = make([]int64, 0)

	var cursor uint64
	pattern := schema.RdbProcessNoncePrefix + "*"

	for {
		// SCAN and get all keys matching the pattern
		keys, newCursor, err := r.rdb.Scan(r.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, nil, err
		}

		for _, key := range keys {
			// get pid
			pid := strings.TrimPrefix(key, schema.RdbProcessNoncePrefix)

			// get nonce
			val, err := r.rdb.Get(r.ctx, key).Result()
			if err != nil {
				log.Error("get nonce failed", "pid", pid, "err", err)
				continue
			}

			// convert nonce to int64
			nonce, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				log.Error("convert nonce failed", "pid", pid, "value", val, "err", err)
				continue
			}

			processIds = append(processIds, pid)
			curNonces = append(curNonces, nonce)
		}

		// if cursor is 0, finished scan
		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	return processIds, curNonces, nil
}

func (r *RDB) GetResult(msgid string) (result *vmmSchema.Result, err error) {
	resultBytes, err := r.rdb.HGet(r.ctx, schema.RdbMsgResult, msgid).Bytes()
	if err != nil {
		if err == redis.Nil {
			log.Warn("can not get result", "msgid", msgid, "err", err)
			err = nil
		}
		return
	}
	result = &vmmSchema.Result{}
	err = json.Unmarshal(resultBytes, result)
	return
}

func (r *RDB) GetResults(pid string, limit int64) (results []vmmSchema.Result, err error) {
	msgids, err := r.rdb.LRange(r.ctx, schema.RdbMsgResultsPrefix+pid, -limit, -1).Result()
	if err != nil {
		if err == redis.Nil {
			log.Warn("can not get results", "pid", pid, "err", err)
			err = nil
		}
		return
	}
	for _, msgid := range msgids {
		r, err := r.GetResult(msgid)
		if err != nil {
			return results, err
		}
		results = append(results, *r)
	}
	return results, nil
}

func (r *RDB) GetCheckpointIndex(pid string) (id string, err error) {
	id, err = r.rdb.Get(r.ctx, schema.RdbCheckpointIndexPrefix+pid).Result()
	if err != nil {
		if err == redis.Nil {
			log.Warn("can not get checkpoint index", "pid", pid, "err", err)
			err = nil
		}
	}
	return
}

func (r *RDB) GetCache(pid, key string) (value string, err error) {
	value, err = r.rdb.Get(r.ctx, schema.RdbCachePrefix+pid+":"+key).Result()
	if err != nil {
		if err == redis.Nil {
			log.Warn("can not get cache", "pid", pid, "key", key, "err", err)
			err = nil
		}
	}
	return
}
