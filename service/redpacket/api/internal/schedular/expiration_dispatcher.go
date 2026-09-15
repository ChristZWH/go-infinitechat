package schedular

import (
	"context"
	"go-infinitechat/common/common"
	luascript "go-infinitechat/service/redpacket/api/internal/script"
	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types/constants"
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
	// KafkaPusherManager 为 nil 时直接拒绝启动，避免静默丢数据
	if svcCtx.KafkaPusherManager == nil {
		common.Error("KafkaPusherManager 未初始化，过期红包扫描调度器无法启动，请检查 Kafka 配置")
		return
	}

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
			return
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
	batchSize := cfg.BatchSize
	maxBatchesPerTick := cfg.MaxBatchesPerTick
	timeBudgetMs := cfg.TimeBudgetMs

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

		// 推送到 Kafka；失败时把 ID 放回 ZSet（score=当前时间），下一轮扫描会重新扫出重试
		for _, redPacketId := range expiredIds {
			if err := svcCtx.KafkaPusherManager.KPush(ctx, constants.TopicRedpacketExpiration, redPacketId, redPacketId); err != nil {
				logx.Errorf("发送红包过期事件到Kafka失败，红包ID: %s, 错误: %v", redPacketId, err)
				_, zaddErr := svcCtx.Redis.Zadd(constants.ExpireZSet, time.Now().UnixMilli(), redPacketId)
				if zaddErr != nil {
					// ZADD 也失败说明 Redis 异常，此 ID 只能靠日志排查（彻底兜底需 outbox 方案）
					logx.Errorf("红包ID放回过期ZSet失败，红包ID: %s, 错误: %v", redPacketId, zaddErr)
				}
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
