package chainkit

import (
	"fmt"
	"strconv"
	"time"

	goarSchema "github.com/permadao/goar/schema"
)

// uploadToChain 实现 Chainkit 上传功能
// 将多个 BundleItem 打包上传到区块链网络
// 返回父交易ID，用于后续状态跟踪
func (c *Chainkit) uploadToChain(items []goarSchema.BundleItem) (parentTxID string, err error) {
	log.Info("uploading bundle items", "count", len(items))
	return c.operator.Upload(items)
}

// 实现 Chainkit 交易聚合功能
// 从 uploadSet 获取所有待上传txid
// 从 idb 获取所有交易
// 使用 goar
// 使用 operator.Upload上传
// 如果成功，从 uploadSet 中移除，加入 uploadingSet（记录父交易ID）
// 如果失败，等待下一次聚合
func (c *Chainkit) aggregate() (string, error) {
	// 收集当前所有待上传的子交易
	c.mu.Lock()
	txids, err := c.GetUploads()
	if err != nil {
		c.mu.Unlock()
		return "", err
	}
	if len(txids) == 0 {
		c.mu.Unlock()
		return "", nil
	}
	c.mu.Unlock()

	items := c.getBundleItems(txids)
	if len(items) == 0 {
		return "", nil
	}

	// 调用 operator.Upload 进行聚合上传（由 operator 实现具体打包逻辑）
	parentTxID, err := c.uploadToChain(items)
	if err != nil {
		return "", err
	}

	// 成功：从待上传移除，加入 uploading 集合
	uploaded := make([]string, len(items))
	for _, item := range items {
		uploaded = append(uploaded, item.Id)
	}
	if err = c.MoveToPending(parentTxID, uploaded); err != nil {
		return "", err
	}

	return parentTxID, nil
}

// 在一个 goroutine 中执行聚合任务
// 按照 aggregationPolicy 进行聚合（仅时间条件）
func (c *Chainkit) tryByTime() {
	interval := c.aggregationPolicy.MaxDelay // second
	ticker := time.NewTicker(interval * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			go c.aggregate()
		}
	}
}

func (c *Chainkit) tryByCount() {
	n, err := c.GetUploadsCount()
	if err != nil {
		return
	}
	if n >= c.aggregationPolicy.MaxItems {
		go c.aggregate()
	}
}

// checkTimeout 检查父交易是否超时（超过1小时未确认）
func (c *Chainkit) checkTimeout(parentTxID string) bool {
	// 从 Redis 获取父交易的上传时间戳
	uploadTimeStr, err := c.redis.HGet(c.ctx, RedisKeyUploadTimestamp, parentTxID).Result()
	if err != nil {
		// 没有记录或获取失败，可能是旧交易，不处理
		return false
	}

	uploadTime, err := strconv.ParseInt(uploadTimeStr, 10, 64)
	if err != nil {
		return false
	}

	// 检查是否超过1小时
	return time.Now().Unix()-uploadTime > 3600
}

// reupload 重新上传超时的父交易
func (c *Chainkit) reupload(parentTxID string) (string, error) {
	log.Debug("reupload parent transaction", "txid", parentTxID)

	// 获取该父交易下的所有子交易ID
	subTxIDs, err := c.GetPendingSub(parentTxID)
	if err != nil {
		return "", fmt.Errorf("failed to get pending sub transactions: %w", err)
	}

	// 重新聚合这些子交易
	items := c.getBundleItems(subTxIDs)
	if len(items) == 0 {
		return "", fmt.Errorf("no valid items to reupload")
	}

	// 重新上传
	newParentTxID, err := c.uploadToChain(items)
	if err != nil {
		log.Error("Failed to reupload parent transaction", "txid", parentTxID, "err", err)
		return "", fmt.Errorf("failed to upload to chain: %w", err)
	}

	// 更新 Redis 记录：移除旧的父交易记录，添加新的
	// 移除旧记录
	if err := c.RemovePending(parentTxID); err != nil {
		return "", fmt.Errorf("failed to remove pending record: %w", err)
	}
	// 添加新记录
	uploaded := make([]string, len(items))
	for _, item := range items {
		uploaded = append(uploaded, item.Id)
	}
	if err := c.MoveToPending(newParentTxID, uploaded); err != nil {
		return "", fmt.Errorf("failed to move to pending: %w", err)
	}

	log.Debug("reuploaded with new parent txid", "txid", newParentTxID)
	return newParentTxID, nil
}

// getBundleItems 从给定的交易ID列表中收集BundleItem数据
func (c *Chainkit) getBundleItems(txids []string) []goarSchema.BundleItem {
	items := make([]goarSchema.BundleItem, 0, len(txids)*2)
	for _, txid := range txids {
		if msg, err := c.node.GetMessage(txid); err == nil && msg != nil {
			items = append(items, *msg)
		}
		if assign, err := c.node.GetAssignByMessage(txid); err == nil && assign != nil {
			items = append(items, *assign)
		}
	}
	return items
}

// 每隔 5 分钟检查交易状态（检查父交易是否确认）
func (c *Chainkit) check() {
	interval := 5 * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// 获取所有待确认的父交易ID
			parentIDs, err := c.GetPendings()
			if err != nil {
				continue
			}
			for _, txid := range parentIDs {
				// 检查是否超时（超过1小时未确认）
				if c.checkTimeout(txid) {
					if _, err := c.reupload(txid); err == nil {
						continue // 已重新上传，跳过当前检查
					}
				}

				ok, err := c.operator.CheckTransaction(txid)
				if err == nil && ok {
					c.RemovePending(txid)
				}
			}
		}
	}
}
