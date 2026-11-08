// Package event_outbox 事务发件箱：把"要发到 Kafka 的领域事件"与业务数据放进同一个 MySQL 事务落库，
// 再由独立的投递器（outbox_dispatcher）异步投递，投递成功才标记 published。
//
// 解决的核心问题：业务改了 Redis/MySQL 之后、发 Kafka 失败时，事件丢失（或反过来业务回滚但事件已发出）。
// 发件箱保证「要么业务 + 事件一起提交，要么一起回滚」，投递失败可重试（消费端需幂等）。
package event_outbox

import (
	"context"
	"fmt"
	"time"

	"go-infinitechat/common/model/txctx"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 投递状态
const (
	StatusPending   int64 = 0 // 待投递
	StatusPublished int64 = 1 // 已投递
)

// TopicRedisPoolInit 是发件箱内部使用的"Redis 红包金额池初始化"事件标记，不是真实 Kafka topic。
// 投递器看到这个 topic 时，不推 Kafka，而是调用 InitRedisPool 重建金额池（幂等）。
const TopicRedisPoolInit = "__redis__pool_init__"

type EventOutbox struct {
	Id          int64     `db:"id"`
	Topic       string    `db:"topic"`
	MessageKey  string    `db:"message_key"`
	DedupKey    string    `db:"dedup_key"`
	Payload     string    `db:"payload"`
	Status      int64     `db:"status"`
	CreatedTime time.Time `db:"created_time"`
	UpdatedTime time.Time `db:"updated_time"`
}

type EventOutboxModel interface {
	// InsertTx 幂等插入一条待投递事件，返回受影响行数（1=新插入，0=dedup_key 冲突被忽略）。
	InsertTx(ctx context.Context, data *EventOutbox) (int64, error)
	FindPending(ctx context.Context, limit int) ([]*EventOutbox, error)
	MarkPublished(ctx context.Context, id int64) (int64, error)
}

type customEventOutboxModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewEventOutboxModel(conn sqlx.SqlConn) EventOutboxModel {
	return &customEventOutboxModel{conn: conn, table: "`event_outbox`"}
}

// InsertTx 在事务内幂等写入一条待投递事件（复用 ctx 里的 tx session）。
// 用 INSERT IGNORE + dedup_key 唯一索引去重：过期扫描会反复扫到同一个红包 ID，重复插入时静默跳过。
func (m *customEventOutboxModel) InsertTx(ctx context.Context, data *EventOutbox) (int64, error) {
	query := fmt.Sprintf(
		"insert ignore into %s (`topic`, `message_key`, `dedup_key`, `payload`, `status`, `created_time`, `updated_time`) values (?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)
	args := []any{data.Topic, data.MessageKey, data.DedupKey, data.Payload, data.Status, data.CreatedTime, data.UpdatedTime}

	var res interface {
		RowsAffected() (int64, error)
	}
	var err error
	if session := txctx.GetSession(ctx); session != nil {
		res, err = session.ExecCtx(ctx, query, args...)
	} else {
		res, err = m.conn.ExecCtx(ctx, query, args...)
	}
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// FindPending 拉取一批待投递事件（按 id 升序，保证顺序）。
func (m *customEventOutboxModel) FindPending(ctx context.Context, limit int) ([]*EventOutbox, error) {
	query := fmt.Sprintf(
		"select `id`, `topic`, `message_key`, `dedup_key`, `payload`, `status`, `created_time`, `updated_time` from %s where `status` = ? order by `id` asc limit ?",
		m.table,
	)
	var rows []*EventOutbox
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, StatusPending, limit); err != nil {
		return nil, err
	}
	return rows, nil
}

// MarkPublished 条件更新为已投递（status=0 → 1），返回受影响行数。
// 用条件更新避免并发投递器重复标记。
func (m *customEventOutboxModel) MarkPublished(ctx context.Context, id int64) (int64, error) {
	query := fmt.Sprintf("update %s set `status` = ? where `id` = ? and `status` = ?", m.table)
	res, err := m.conn.ExecCtx(ctx, query, StatusPublished, id, StatusPending)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
