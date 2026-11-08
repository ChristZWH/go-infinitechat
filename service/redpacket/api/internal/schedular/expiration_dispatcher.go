package schedular

import (
	"context"
	"go-infinitechat/common/common"
	luascript "go-infinitechat/service/redpacket/api/internal/script"
	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types/constants"
	"go-infinitechat/service/redpacket/model/event_outbox"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// 分布式锁 key，集群中多个实例竞争同一把锁，保证同一时刻只有一个实例执行扫描任务
	lockKeyExpirationDispatcher = "lock:expiration-dispatcher"
	// 分布式锁过期时间（秒），go-zero RedisLock 最小粒度为秒
	lockExpireSeconds = 1
)

// 启动过期红包扫描调度器，该方法会阻塞当前 goroutine，应通过 go 关键字在独立 goroutine 中启动
func StartExpirationDispatcher(ctx context.Context, svcCtx *svc.ServiceContext) {
	// Script 对象提到循环外，只创建一次
	scanScript := redis.NewScript(luascript.ScanExpiredRedPacketsLua)

	// 每秒触发一次
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	common.Info("过期红包扫描调度器已启动")

	for {
		select {
		case <-ctx.Done():
			// 收到关闭信号
			common.Info("过期红包扫描调度器已停止")
			return // 函数退出调用 ticker.Stop()
		case <-ticker.C:
			// 每一秒触发一次扫描调度
			dispatch(ctx, svcCtx, scanScript)
		}
	}
}

// 单次调度：竞争分布式锁 -> 批量扫描 -> 投递 Kafka
func dispatch(ctx context.Context, svcCtx *svc.ServiceContext, scanScript *redis.Script) {
	defer func() {
		if r := recover(); r != nil {
			logx.Errorf("扫描过期红包任务执行失败: %v", r)
		}
	}()

	// 分布式锁
	// 使用 go-zero RedisLock
	lock := redis.NewRedisLock(svcCtx.Redis, lockKeyExpirationDispatcher)
	lock.SetExpire(lockExpireSeconds)

	acquired, err := lock.AcquireCtx(ctx)
	if err != nil {
		logx.Errorf("获取分布式锁失败: %v", err)
		return
	}
	if !acquired {
		// 其他实例已在执行，跳过本轮
		return
	}
	defer lock.ReleaseCtx(ctx)

	// 参数从 config 读取
	cfg := svcCtx.Config.ExpirationDispatcher
	batchSize := cfg.BatchSize                 //没批最多扫描红包数
	maxBatchesPerTick := cfg.MaxBatchesPerTick // 每轮最多处理批次数
	timeBudgetMs := cfg.TimeBudgetMs           // 单次扫描最大时间

	startTime := time.Now().UnixMilli()
	totalExpiredCount := 0
	batches := 0

	for batches < maxBatchesPerTick && (time.Now().UnixMilli()-startTime) < timeBudgetMs {
		result, err := svcCtx.Redis.ScriptRunCtx(ctx, scanScript, []string{constants.ExpireZSet}, "0", strconv.Itoa(batchSize))
		if err != nil {
			logx.Errorf("执行过期扫描Lua脚本失败: %v", err)
			break
		}

		expiredIds := parseStringList(result)
		if len(expiredIds) == 0 {
			break
		}

		// 过期事件走事务发件箱：INSERT IGNORE 幂等落库（dedup_key=expire:<id>），
		// 由 outbox_dispatcher 投递 Kafka；成功登记后才 ZREM 摘除 ZSet。
		// 插入失败（MySQL 挂）不摘除，下轮重扫重插。
		for _, redPacketId := range expiredIds {
			if err := enqueueExpirationEvent(ctx, svcCtx, redPacketId); err != nil {
				logx.Errorf("写过期事件到发件箱失败，红包ID: %s, 错误: %v（下轮重试）", redPacketId, err)
				continue
			}
			// 已登记进发件箱（无论新插入还是去重），从 ZSet 摘除清理；
			// ZREM 失败也安全：下轮重扫，INSERT IGNORE 去重后再次尝试摘除。
			if _, err := svcCtx.Redis.Zrem(constants.ExpireZSet, redPacketId); err != nil {
				logx.Errorf("从过期ZSet摘除失败，红包ID: %s, 错误: %v", redPacketId, err)
			}
		}

		totalExpiredCount += len(expiredIds)
		batches++
		logx.Infof("第%d批扫描完成，本批到期红包数: %d", batches, len(expiredIds))

		if len(expiredIds) < batchSize {
			break
		}
	}

	if totalExpiredCount > 0 {
		logx.Infof("过期红包扫描完成，总共处理: %d 个，耗时: %d ms，批次数: %d", totalExpiredCount, time.Now().UnixMilli()-startTime, batches)
	}
}

// enqueueExpirationEvent 把过期红包事件幂等写入发件箱（dedup_key=expire:<id>）。
// 由 outbox_dispatcher 投递到 Kafka，ExpirationConsumer 消费退款（幂等）。
func enqueueExpirationEvent(ctx context.Context, svcCtx *svc.ServiceContext, redPacketId string) error {
	now := time.Now()
	_, err := svcCtx.EventOutboxModel.InsertTx(ctx, &event_outbox.EventOutbox{
		Topic:       constants.TopicRedpacketExpiration,
		MessageKey:  redPacketId,             // key = 红包ID
		DedupKey:    "expire:" + redPacketId, // 幂等去重：扫描重扫时 INSERT IGNORE 跳过
		Payload:     redPacketId,             // payload = 红包ID（ExpirationConsumer 直接 parse int64）
		Status:      event_outbox.StatusPending,
		CreatedTime: now,
		UpdatedTime: now,
	})
	return err
}

func parseStringList(result interface{}) []string {
	if result == nil {
		return nil
	}
	list, ok := result.([]interface{})
	if !ok {
		return nil
	}
	strs := make([]string, 0, len(list))
	for _, item := range list {
		switch s := item.(type) {
		case string:
			strs = append(strs, s)
		case []byte:
			strs = append(strs, string(s))
		}
	}
	return strs
}
