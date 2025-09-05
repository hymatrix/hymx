package chainkit

import "time"

const (
	RedisUrl                = "redis://@localhost:6379/0"
	RedisKeyUploads         = "chainkit:uploads"
	RedisKeyPendingPrefix   = "chainkit:pending:"          // Hash key prefix: chainkit:pending:{parentTxID}
	RedisKeyUploadTimestamp = "chainkit:upload_timestamps" // Hash key for storing upload timestamps
)

// Redis 操作封装函数

// GetUploads 获取待上传集合中的所有成员
func (c *Chainkit) GetUploads() ([]string, error) {
	return c.redis.SMembers(c.ctx, RedisKeyUploads).Result()
}

// GetUploadsCount 获取待上传集合的成员数量
func (c *Chainkit) GetUploadsCount() (int64, error) {
	return c.redis.SCard(c.ctx, RedisKeyUploads).Result()
}

// AddToUploads 添加交易ID到待上传集合
func (c *Chainkit) AddToUploads(txid string) error {
	return c.redis.SAdd(c.ctx, RedisKeyUploads, txid).Err()
}

// MoveToPending 从待上传集合移除交易ID并添加到待确认Hash集合
func (c *Chainkit) MoveToPending(parentTxID string, txids []string) error {
	pipe := c.redis.TxPipeline()
	for _, txid := range txids {
		pipe.SRem(c.ctx, RedisKeyUploads, txid)
		// 将子txid添加到以parentTxID为key的Hash中
		pipe.HSet(c.ctx, RedisKeyPendingPrefix+parentTxID, txid, "1")
	}
	// 记录父交易的上传时间戳
	pipe.HSet(c.ctx, RedisKeyUploadTimestamp, parentTxID, time.Now().Unix())
	_, err := pipe.Exec(c.ctx)
	return err
}

// GetPendings 获取所有待确认的父交易ID
func (c *Chainkit) GetPendings() ([]string, error) {
	keys, err := c.redis.Keys(c.ctx, RedisKeyPendingPrefix+"*").Result()
	if err != nil {
		return nil, err
	}
	// 移除前缀，只返回parentTxID
	parentTxIDs := make([]string, len(keys))
	for i, key := range keys {
		parentTxIDs[i] = key[len(RedisKeyPendingPrefix):]
	}
	return parentTxIDs, nil
}

// GetPendingSub 根据父交易ID获取所有子交易ID
func (c *Chainkit) GetPendingSub(parentTxID string) ([]string, error) {
	return c.redis.HKeys(c.ctx, RedisKeyPendingPrefix+parentTxID).Result()
}

// RemovePending 移除整个父交易及其所有子交易
func (c *Chainkit) RemovePending(parentTxID string) error {
	pipe := c.redis.TxPipeline()
	// 删除待确认记录
	pipe.Del(c.ctx, RedisKeyPendingPrefix+parentTxID)
	// 清理上传时间记录
	pipe.HDel(c.ctx, RedisKeyUploadTimestamp, parentTxID)
	_, err := pipe.Exec(c.ctx)
	return err
}
