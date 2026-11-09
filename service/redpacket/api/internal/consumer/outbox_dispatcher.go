package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-infinitechat/common/kafka"
	"go-infinitechat/service/redpacket/api/internal/svc"
	"go-infinitechat/service/redpacket/api/internal/types/constants"
	"go-infinitechat/service/redpacket/model/event_outbox"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	outboxPollInterval = time.Second // 轮询间隔：发件箱是异步投递，秒级延迟可接受
	outboxBatchSize    = 100
)

// StartOutboxDispatcher 启动事务发件箱投递器：
// 轮询 status=0 的事件 → 按类型处理（推 Kafka 或重建 Redis 金额池）→ 成功标记已投递。
//
// 发件箱分派关系（谁写 → 投递器 → 谁消费）：
//
//	写入口                           topic                      投递器动作           最终消费者
//	─────────────────────────────────────────────────────────────────────────────────────────
//	sendredpacketlogic（发红包）      store-topic                 pushKafkaEvent       offline 消费落库
//	sendredpacketlogic（发红包）      message-topic               pushKafkaEvent       realtime 消费推送
//	sendredpacketlogic（发红包）      __redis__pool_init__        handlePoolInitEvent  投递器自处理（重建金额池）
//	expiration_dispatcher（过期扫描） redpacket-expiration-topic  pushKafkaEvent       ExpirationConsumer 退款
//
//	注意：抢红包入账不走这里——它走 Redis Stream（receive_stream_worker），
//	只有领完事件由 stream worker 直接推 Kafka，也不经过发件箱。
//
// at-least-once：处理失败保留 pending 下轮重试；消费端（offline/realtime）与池初始化均幂等。
// 注意：PusherManager 必须用 WithSyncPush 创建，否则 PushRaw 返回 nil 只代表"入内存队列"，
// 后台真写失败会静默丢——投递器会误判并误标 published。
func StartOutboxDispatcher(svcCtx *svc.ServiceContext) {
	go runOutboxDispatcher(svcCtx)
}

func runOutboxDispatcher(svcCtx *svc.ServiceContext) {
	pusher := svcCtx.KafkaPusherManager
	if pusher == nil {
		logx.Errorf("Kafka PusherManager 未初始化：Kafka 事件将滞留 pending（Redis 池初始化事件不受影响）")
	}

	for {
		pending, err := svcCtx.EventOutboxModel.FindPending(context.Background(), outboxBatchSize)
		if err != nil {
			logx.Errorf("查询待投递事件失败: %v", err)
			time.Sleep(outboxPollInterval)
			continue
		}
		if len(pending) == 0 {
			time.Sleep(outboxPollInterval)
			continue
		}

		for _, ev := range pending {
			var procErr error
			if ev.Topic == event_outbox.TopicRedisPoolInit {
				procErr = handlePoolInitEvent(svcCtx, ev.Payload)
			} else {
				procErr = pushKafkaEvent(pusher, ev)
			}

			if procErr != nil {
				// 留 pending，下轮重试
				logx.Errorf("发件箱处理失败, id=%d, topic=%s: %v", ev.Id, ev.Topic, procErr)
				continue
			}
			if _, err := svcCtx.EventOutboxModel.MarkPublished(context.Background(), ev.Id); err != nil {
				logx.Errorf("发件箱标记已投递失败, id=%d: %v", ev.Id, err)
			}
		}
	}
}

// pushKafkaEvent 把事件推送到 Kafka（同步，返回真实写结果）。
func pushKafkaEvent(pusher *kafka.PusherManager, ev *event_outbox.EventOutbox) error {
	if pusher == nil {
		return fmt.Errorf("Kafka PusherManager 未初始化")
	}
	if ev.MessageKey == "" {
		return pusher.PushRaw(context.Background(), ev.Topic, ev.Payload)
	}
	return pusher.KPushRaw(context.Background(), ev.Topic, ev.MessageKey, ev.Payload)
}

// handlePoolInitEvent 处理「重建红包金额池」事件：解析 payload → InitRedisPool（幂等）。
// 这是 InitRedisPool 同步失败后的异步补偿，保证钱已扣的红包最终能建出金额池。
func handlePoolInitEvent(svcCtx *svc.ServiceContext, payload string) error {
	var p struct {
		RedPacketId     int64   `json:"redPacketId"`
		Amounts         []int64 `json:"amounts"`
		ExpireTimestamp int64   `json:"expireTimestamp"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return err
	}

	// 防「复活」：红包已领完/已过期时，Redis 池已被 CleanRedisCache 清掉（含 records、ZSet member）。
	// 若此时重建满额池 + ZADD NX 重新登记，会让已终态的红包被再次抢 → 凭空多出钱（双花）。
	// 先查 DB 状态，非「未领完」一律跳过建池（return nil 让事件标 published）。
	rp, err := svcCtx.RedPacketModel.FindOne(context.Background(), p.RedPacketId)
	if err != nil {
		return err // 查不到 / DB 异常 → 保留 pending 重试
	}
	if rp.Status != constants.StatusNotCompleted {
		logx.Infof("红包非 未领完 状态，跳过建池: redPacketId=%d, status=%d", p.RedPacketId, rp.Status)
		return nil
	}

	// 用发送时刻的原始 expireTimestamp，ZADD NX 不覆盖，避免停摆重启后红包被"续命"
	return svcCtx.RedPacketService.InitRedisPool(context.Background(), p.RedPacketId, p.Amounts, p.ExpireTimestamp)
}
