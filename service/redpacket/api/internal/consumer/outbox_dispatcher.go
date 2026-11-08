package consumer

import (
	"context"
	"time"

	"go-infinitechat/service/redpacket/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	outboxPollInterval = time.Second // 轮询间隔：发件箱是异步投递，秒级延迟可接受
	outboxBatchSize    = 100
)

// StartOutboxDispatcher 启动事务发件箱投递器：
// 轮询 status=0 的事件 → 推 Kafka（同步）→ 成功标记已投递。
//
// at-least-once：投递失败保留 pending 下轮重试；消费端（offline/realtime）本身幂等。
// 注意：PusherManager 必须用 WithSyncPush 创建，否则 PushRaw 返回 nil 只代表"入内存队列"，
// 后台真写失败会静默丢——投递器会误判并误标 published。
func StartOutboxDispatcher(svcCtx *svc.ServiceContext) {
	go runOutboxDispatcher(svcCtx)
}

func runOutboxDispatcher(svcCtx *svc.ServiceContext) {
	pusher := svcCtx.KafkaPusherManager
	if pusher == nil {
		logx.Error("Kafka PusherManager 未初始化，发件箱投递器不启动")
		return
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
			var pushErr error
			if ev.MessageKey == "" {
				pushErr = pusher.PushRaw(context.Background(), ev.Topic, ev.Payload)
			} else {
				pushErr = pusher.KPushRaw(context.Background(), ev.Topic, ev.MessageKey, ev.Payload)
			}
			if pushErr != nil {
				// 留 pending，下轮重试
				logx.Errorf("发件箱投递失败, id=%d, topic=%s: %v", ev.Id, ev.Topic, pushErr)
				continue
			}
			if _, err := svcCtx.EventOutboxModel.MarkPublished(context.Background(), ev.Id); err != nil {
				logx.Errorf("发件箱标记已投递失败, id=%d: %v", ev.Id, err)
			}
		}
	}
}
