package consumer

import (
	"context"
	"strconv"
	"time"

	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types/constants"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// StartReceiveStreamWorker 启动领取事件发件箱的消费者。
//
// 抢包 Lua 在 LPOP + HSET 的同时，把领取事件 XADD 进 Redis Stream（原子）。
// 这里 XREADGROUP 消费 → SettleReceive 入账 → XACK。
//
// at-least-once：入账失败不 XACK，消息留在 pending 里，消费组稍后重投；
// 入账本身有幂等检查（同一红包同一用户），重投不会重复加钱。
func StartReceiveStreamWorker(svcCtx *svc.ServiceContext) {
	go runReceiveStreamWorker(svcCtx)
}

func runReceiveStreamWorker(svcCtx *svc.ServiceContext) {
	// 一、准备阶段
	rds := svcCtx.Redis
	stream := constants.ReceiveStreamKey
	group := constants.ReceiveStreamGroup
	consumerId := "worker-1" // 单实例消费；多实例时应按 host 唯一化

	// 1. 阻塞命令（XREADGROUP BLOCK）必须用专用 node，避免占满 公共连接池
	node, err := redis.CreateBlockingNode(rds)
	if err != nil {
		logx.Errorf("创建 Redis 阻塞节点失败: %v", err)
		return
	}
	defer node.Close()

	// 2. 创建消费组（已存在时返回 BUSYGROUP，忽略即可）；可以有多个消费者 Group （各 消费者Group 之间相互独立），本项目没有这么做
	// '$' 从新的 Group的起始位置开始，重启可能会丢失数据; '0' 从头开始读取
	_, _ = rds.XGroupCreateMkStream(stream, group, "0")

	// 二、循环阶段
	// 死循环，所以需要 go 开启异步 goroutine
	// 死循环——读新消息 → 逐条入账 → 成功 XACK 签收、失败留在 pending 等重投 → 回到读
	for {
		// 1. 读新消息（阻塞最多 5 秒；无新消息返回 red.Nil，属正常）
		// ">" 表示只读新消息；pending 失败消息的兜底见下方; 一次最多读 100 条；false: 读完不自动签收 -> 签收手动 XACK 控制
		newStreams, err := rds.XReadGroup(node, group, consumerId, 100, 5*time.Second, false, stream, ">")
		if err == nil {
			processStreams(svcCtx, rds, stream, group, newStreams)
		} else if err != red.Nil {
			logx.Errorf("XReadGroup(新消息) 失败: %v", err)
		}

		// 2. 重读自己 PEL 里的未确认消息（id "0"，block=0 不阻塞）：
		//    SettleReceive 失败、未 XACK 的消息会留在该消费者的 PEL 里，
		//    这一读把它们重新拉回来处理，实现"重试直到成功"。
		//    go-zero 没有 XAUTOCLAIM/XCLAIM，单消费者用 "0" 重读自己的 PEL 是标准做法。
		pendingStreams, err := rds.XReadGroup(node, group, consumerId, 100, 0, false, stream, "0")
		if err == nil {
			processStreams(svcCtx, rds, stream, group, pendingStreams)
		} else if err != red.Nil {
			logx.Errorf("XReadGroup(pending) 失败: %v", err)
		}
	}
}

// processStreams 逐条处理 Stream 消息：入账成功 XACK，失败留在 pending 等下一轮重读。
func processStreams(svcCtx *svc.ServiceContext, rds *redis.Redis, stream, group string, streams []red.XStream) {
	for _, s := range streams {
		for _, msg := range s.Messages {
			if err := handleReceiveEvent(svcCtx, msg.Values); err != nil {
				logx.Errorf("领取事件入账失败, id=%s: %v", msg.ID, err)
				continue
			}
			if _, err := rds.XAck(stream, group, msg.ID); err != nil {
				logx.Errorf("XACK 失败, id=%s: %v", msg.ID, err)
			}
		}
	}
}

func handleReceiveEvent(svcCtx *svc.ServiceContext, values map[string]interface{}) error {
	return svcCtx.RedPacketService.SettleReceive(
		context.Background(),
		streamValueToInt64(values["userId"]),
		streamValueToInt64(values["redPacketId"]),
		streamValueToInt64(values["amount"]),
		streamValueToInt64(values["receivedTime"]),
	)
}

// streamValueToInt64 Redis Stream 的 field value 都是字符串（Lua XADD 写入的也是字符串）。
func streamValueToInt64(v interface{}) int64 {
	switch val := v.(type) {
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	case int64:
		return val
	case int:
		return int64(val)
	default:
		return 0
	}
}
