package consumer

import (
	"context"
	"encoding/json"
	"go-infinitechat/common/common"
	"go-infinitechat/common/model/txctx"
	"go-infinitechat/common/utils"
	"go-infinitechat/service/redpacket/api/internal/config"
	"go-infinitechat/service/redpacket/api/internal/service"
	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types/constants"
	"go-infinitechat/service/redpacket/model/red_packet_receive"
	"strconv"
	"time"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
)

type ReceiveConsumer struct {
	rpSvc *service.RedPacketService
}

func StartConsumer(svcCtx *svc.ServiceContext, c config.Config) []func() {
	if len(c.Kafka.Brokers) == 0 {
		logx.Error("Kafka Brokers 未配置，消费者不启动")
		return nil
	}
	rp_svc := svcCtx.RedPacketService
	var stopFunc []func()

	// 1. 红包领取事件消费者
	receiveConsumer := &ReceiveConsumer{rpSvc: rp_svc}
	receiveQueue := kq.MustNewQueue(kq.KqConf{
		ServiceConf: c.ServiceConf,
		Brokers:     c.Kafka.Brokers,
		Group:       "redpacket-receive-handler-group",
		Topic:       constants.TopicRedpacketReceive,
		Offset:      "last",
		Consumers:   3,
		Processors:  3,
	}, kq.WithHandle(receiveConsumer.Consume))
	go receiveQueue.Start()
	stopFunc = append(stopFunc, receiveQueue.Stop)

	// 2. 红包领完事件消费者
	completedConsumer := &CompletedConsumer{rpSvc: rp_svc}
	completedQueue := kq.MustNewQueue(kq.KqConf{
		ServiceConf: c.ServiceConf,
		Brokers:     c.Kafka.Brokers,
		Group:       "redpacket-completed-handler-group",
		Topic:       constants.TopicRedpacketCompleted,
		Offset:      "last",
		Consumers:   3,
		Processors:  3,
	}, kq.WithHandle(completedConsumer.Consume))
	go completedQueue.Start()
	stopFunc = append(stopFunc, completedQueue.Stop)

	// 3. 红包过期事件消费者
	expirationConsumer := &ExpirationConsumer{rpSvc: rp_svc}
	expirationQueue := kq.MustNewQueue(kq.KqConf{
		ServiceConf: c.ServiceConf,
		Brokers:     c.Kafka.Brokers,
		Group:       "expiration-task-executor-group",
		Topic:       constants.TopicRedpacketExpiration,
		Offset:      "last",
		Consumers:   3,
		Processors:  3,
	}, kq.WithHandle(expirationConsumer.Consume))
	go expirationQueue.Start()
	stopFunc = append(stopFunc, expirationQueue.Stop)

	return stopFunc
}

func (c *ReceiveConsumer) Consume(ctx context.Context, key, value string) error {
	logx.Infof("收到红包领取事件：%s", value)
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(value), &event); err != nil {
		logx.Errorf("解析红包领取事件失败: %v", err)
		return nil
	}

	userId := jsonToInt64(event["userId"])
	redPacketId := jsonToInt64(event["redPacketId"])
	receivedAmount := jsonToInt64(event["receivedAmount"])
	receivedTime := jsonToInt64(event["receivedTime"])

	// 幂等性检查
	existing, err := c.rpSvc.RedPacketReceiveModel.FindOneByRedPacketIdReceiverId(ctx, redPacketId, userId)
	if err == nil && existing != nil {
		logx.Infof("红包领取记录已存在，跳过。红包ID: %d, 用户ID: %d", redPacketId, userId)
		return nil
	}

	now := time.Now()
	// 开启事务
	err = txctx.WithTransaction(ctx, c.rpSvc.SqlConn, func(ctx context.Context) error {
		// 1. 插入领取记录
		_, err = c.rpSvc.RedPacketReceiveModel.InsertTx(ctx, &red_packet_receive.RedPacketReceive{
			RedPacketReceiveId: utils.NextInt(),
			RedPacketId:        redPacketId,
			ReceiverId:         userId,
			Amount:             receivedAmount,
			ReceivedAt:         time.UnixMilli(receivedTime),
			CreatedTime:        now,
			UpdatedTime:        now,
		})
		common.ThrowIfWithMsg(err != nil, common.MysqlError, "插入领取记录失败", err)

		// 2. 增加余额
		err = c.rpSvc.AddBalance(ctx, userId, receivedAmount)
		common.ThrowIfWithMsg(err != nil, common.OperationError, "增加余额失败", err)

		// 3. 记录余额变动日志
		c.rpSvc.InsertBalanceLog(ctx, userId, receivedAmount, constants.BalanceLogTypeReceive, redPacketId)
		return nil
	})

	logx.Infof("红包领取记录处理完成。红包ID: %d, 用户ID: %d, 金额: %d", redPacketId, userId, receivedAmount)
	if err != nil {
		return err
	}
	return nil
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

	// 2. 计算剩余余额
	remainAmount := c.rpSvc.CalculateRemainAmount(ctx, redPacketId)

	// 开启事务
	err = txctx.WithTransaction(ctx, c.rpSvc.SqlConn, func(ctx context.Context) error {
		// 3. 退回给发送者
		if remainAmount > 0 {
			err = c.rpSvc.AddBalance(ctx, rp.SenderId, remainAmount)
			common.ThrowIfWithMsg(err != nil, common.SystemError, "AddBalance Failed 退钱失败", err)
			c.rpSvc.InsertBalanceLog(ctx, rp.SenderId, remainAmount, constants.BalanceLogTypeRefund, redPacketId)
			logx.Infof("红包过期，退回金额: %d，用户ID: %d", remainAmount, rp.SenderId)
		}

		// 4. 更新红包状态为已过期
		rp.Status = constants.StatusExpired
		rp.UpdatedTime = time.Now()
		err = c.rpSvc.RedPacketModel.UpdateTx(ctx, rp)
		common.ThrowIfWithMsg(err != nil, common.OperationError, "更新红包状态失败Consume-WithTransaction", err)

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
