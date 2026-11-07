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
		// ">" 表示只读新消息；pending 消息由 XAck 失败后通过 XAUTOCLAIM 兜底（本实现简化，靠重投幂等）; 一次最多读 100 条；false: 读完不自动签收 -> 签收手动 XACK 控制
		streams, err := rds.XReadGroup(node, group, consumerId, 100, 5*time.Second, false, stream, ">")
		if err != nil {
			if err != red.Nil { // red.Nil = 5 秒内无新消息，属正常
				logx.Errorf("XReadGroup 失败: %v", err)
			}
			continue
		}

		for _, s := range streams {
			for _, msg := range s.Messages {
				if err := handleReceiveEvent(svcCtx, msg.Values); err != nil {
					// 入账失败不 XACK，等待重投
					logx.Errorf("领取事件入账失败, id=%s: %v", msg.ID, err)
					continue
				}
				if _, err := rds.XAck(stream, group, msg.ID); err != nil {
					logx.Errorf("XACK 失败, id=%s: %v", msg.ID, err)
				}
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
