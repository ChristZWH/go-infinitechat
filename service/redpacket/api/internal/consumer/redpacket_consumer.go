package consumer

import (
	"context"
	"encoding/json"
	"go-infinitechat/common/common"
	"go-infinitechat/common/model/txctx"
	"go-infinitechat/service/redpacket/api/internal/config"
	"go-infinitechat/service/redpacket/api/internal/service"
	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types/constants"
	"strconv"
	"time"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
)

// StartConsumer 启动红包 Kafka 消费者。
// 注意：领取入账已改走 Redis Stream 发件箱（见 receive_stream_worker.go），
// 这里只保留「领完」「过期」两个事件消费者。
func StartConsumer(svcCtx *svc.ServiceContext, c config.Config) []func() {
	if len(c.Kafka.Brokers) == 0 {
		logx.Error("Kafka Brokers 未配置，消费者不启动")
		return nil
	}
	rpSvc := svcCtx.RedPacketService
	var stopFunc []func()

	// 1. 红包领完事件消费者
	completedConsumer := &CompletedConsumer{rpSvc: rpSvc}
	completedQueue := kq.MustNewQueue(kq.KqConf{
		ServiceConf: c.ServiceConf,
		Brokers:     c.Kafka.Brokers,
		Group:       "redpacket-completed-handler-group",
		Topic:       constants.TopicRedpacketCompleted,
		Offset:      "last",
		// 关键：kq 的 ForceCommit 默认为 true，即 handler 返回 error 也会提交 offset。
		// 显式关掉，落库失败时至少不会立刻把位点推过去（配合 consumer 内的有限重试）。
		ForceCommit: false,
		Consumers:   3,
		Processors:  3,
	}, kq.WithHandle(completedConsumer.Consume))
	go completedQueue.Start()
	stopFunc = append(stopFunc, completedQueue.Stop)

	// 2. 红包过期事件消费者
	expirationConsumer := &ExpirationConsumer{rpSvc: rpSvc}
	expirationQueue := kq.MustNewQueue(kq.KqConf{
		ServiceConf: c.ServiceConf,
		Brokers:     c.Kafka.Brokers,
		Group:       "expiration-task-executor-group",
		Topic:       constants.TopicRedpacketExpiration,
		Offset:      "last",
		// 关键：kq 的 ForceCommit 默认为 true，即 handler 返回 error 也会提交 offset。
		// 显式关掉，落库失败时至少不会立刻把位点推过去（配合 consumer 内的有限重试）。
		ForceCommit: false,
		Consumers:   3,
		Processors:  3,
	}, kq.WithHandle(expirationConsumer.Consume))
	go expirationQueue.Start()
	stopFunc = append(stopFunc, expirationQueue.Stop)

	return stopFunc
}

type CompletedConsumer struct {
	rpSvc *service.RedPacketService
}

func (c *CompletedConsumer) Consume(ctx context.Context, key, value string) error {
	logx.Infof("收到红包领完事件: %s", value)
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(value), &event); err != nil {
		logx.Errorf("解析红包领完事件失败: %v", err)
		return nil
	}

	redPacketId := jsonToInt64(event["redPacketId"])

	// 1. 更新红包状态
	rp, err := c.rpSvc.RedPacketModel.FindOne(ctx, redPacketId)
	if err != nil {
		logx.Infof("红包不存在，红包ID: %d", redPacketId)
		return err
	}
	rp.Status = constants.StatusCompleted
	rp.UpdatedTime = time.Now()
	// 单条 SQL 天生原子，不用添加事务
	err = c.rpSvc.RedPacketModel.Update(ctx, rp)
	if err != nil {
		return err
	}

	// 2. 清理 Redis 缓存
	c.rpSvc.CleanRedisCache(redPacketId)

	logx.Infof("红包已领取完，清理完成。红包ID: %d", redPacketId)
	return nil
}

type ExpirationConsumer struct {
	rpSvc *service.RedPacketService
}

func (c *ExpirationConsumer) Consume(ctx context.Context, key, value string) error {
	logx.Infof("收到红包过期事件: %s", value)
	redPacketId, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		var event map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(value), &event); jsonErr == nil {
			redPacketId = jsonToInt64(event["redPacketId"])
		} else {
			logx.Errorf("解析红包过期事件失败: %v", err)
			return nil
		}
	}

	// 1. 查询红包信息
	rp, err := c.rpSvc.RedPacketModel.FindOne(ctx, redPacketId)
	if err != nil {
		return err
	}
	if rp.Status == constants.StatusExpired {
		return nil // 已退过，幂等跳过
	}
	if rp.Status != constants.StatusNotCompleted {
		logx.Infof("红包状态不是未领取完，跳过。红包ID: %d, 状态: %d", redPacketId, rp.Status)
		return nil
	}

	// 2. 计算剩余余额（Redis 出错必须上抛，吞成 0 会导致"状态已过期但一分未退"）
	remainAmount, calcErr := c.rpSvc.CalculateRemainAmount(ctx, redPacketId)
	if calcErr != nil {
		return calcErr // 交给 kq 重试
	}

	// 开启事务
	err = txctx.WithTransaction(ctx, c.rpSvc.SqlConn, func(ctx context.Context) error {
		// 3. 先原子抢占「未领完 → 已过期」这一状态迁移，抢到的那一次才退款。
		//    只 UPDATE 不先查状态：Kafka 重投或并发消费时 RowsAffected 为 0，钱只会退一次。
		affected, updErr := c.rpSvc.RedPacketModel.MarkExpiredIfNotCompleted(ctx, redPacketId, constants.StatusExpired, constants.StatusNotCompleted)
		common.ThrowIfWithMsg(updErr != nil, common.OperationError, "更新红包状态失败Consume-WithTransaction", updErr)
		if affected == 0 {
			logx.Infof("红包已被其它消费者处理，跳过退款。红包ID: %d", redPacketId)
			return nil
		}

		// 4. 退回给发送者
		if remainAmount > 0 {
			if addErr := c.rpSvc.AddBalance(ctx, rp.SenderId, remainAmount); addErr != nil {
				common.ThrowIfWithMsg(true, common.SystemError, "AddBalance Failed 退钱失败", addErr)
			}
			c.rpSvc.InsertBalanceLog(ctx, rp.SenderId, remainAmount, constants.BalanceLogTypeRefund, redPacketId)
			logx.Infof("红包过期，退回金额: %d，用户ID: %d", remainAmount, rp.SenderId)
		}

		return nil
	})
	if err != nil {
		return err // 触发 kq 重试
	}
	// 5. 清理缓存

	c.rpSvc.CleanRedisCache(redPacketId)
	logx.Infof("红包过期处理完成。红包ID: %d", redPacketId)
	return nil
}

func jsonToInt64(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case json.Number:
		n, _ := val.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	case int64:
		return val
	default:
		return 0
	}
}
