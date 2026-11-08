package event_outbox

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func newTestModel(t *testing.T) (sqlx.SqlConn, sqlmock.Sqlmock, *customEventOutboxModel) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	conn := sqlx.NewSqlConnFromDB(db)
	m := &customEventOutboxModel{conn: conn, table: "`event_outbox`"}
	return conn, mock, m
}

func sampleEvent() *EventOutbox {
	now := time.Now()
	return &EventOutbox{
		Topic:       "store-topic",
		MessageKey:  "",
		DedupKey:    "store:123",
		Payload:     `{"sessionId":1,"senderId":2,"type":3}`,
		Status:      StatusPending,
		CreatedTime: now,
		UpdatedTime: now,
	}
}

// InsertTx 非事务分支：INSERT IGNORE，返回 RowsAffected（1=新插入）
func TestInsertTxNonTransactional(t *testing.T) {
	_, mock, m := newTestModel(t)
	ev := sampleEvent()

	mock.ExpectExec("insert ignore into `event_outbox` (`topic`, `message_key`, `dedup_key`, `payload`, `status`, `created_time`, `updated_time`) values (?, ?, ?, ?, ?, ?, ?)").
		WithArgs(ev.Topic, ev.MessageKey, ev.DedupKey, ev.Payload, ev.Status, ev.CreatedTime, ev.UpdatedTime).
		WillReturnResult(sqlmock.NewResult(1, 1))

	affected, err := m.InsertTx(context.Background(), ev)
	if err != nil {
		t.Fatalf("InsertTx: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, 期望 1（新插入）", affected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("未满足的 SQL 期望: %v", err)
	}
}

// InsertTx 幂等：dedup_key 冲突时 INSERT IGNORE 返回 affected=0
func TestInsertTxIgnoresDuplicate(t *testing.T) {
	_, mock, m := newTestModel(t)
	ev := sampleEvent()

	mock.ExpectExec("insert ignore into `event_outbox` (`topic`, `message_key`, `dedup_key`, `payload`, `status`, `created_time`, `updated_time`) values (?, ?, ?, ?, ?, ?, ?)").
		WithArgs(ev.Topic, ev.MessageKey, ev.DedupKey, ev.Payload, ev.Status, ev.CreatedTime, ev.UpdatedTime).
		WillReturnResult(sqlmock.NewResult(0, 0))

	affected, err := m.InsertTx(context.Background(), ev)
	if err != nil {
		t.Fatalf("InsertTx: %v", err)
	}
	if affected != 0 {
		t.Fatalf("affected = %d, 期望 0（重复被忽略）", affected)
	}
}

// FindPending 拉取待投递事件，按 id 升序
func TestFindPending(t *testing.T) {
	_, mock, m := newTestModel(t)

	now := time.Now()
	mock.ExpectQuery("select `id`, `topic`, `message_key`, `dedup_key`, `payload`, `status`, `created_time`, `updated_time` from `event_outbox` where `status` = ? order by `id` asc limit ?").
		WithArgs(StatusPending, 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "topic", "message_key", "dedup_key", "payload", "status", "created_time", "updated_time"}).
			AddRow(1, "store-topic", "", "store:1", `{}`, StatusPending, now, now).
			AddRow(2, "message-topic", "9", "push:2", `{}`, StatusPending, now, now))

	rows, err := m.FindPending(context.Background(), 100)
	if err != nil {
		t.Fatalf("FindPending: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("返回 %d 条, 期望 2", len(rows))
	}
	if rows[0].Id != 1 || rows[1].Id != 2 {
		t.Fatalf("应按 id 升序, 得到 %d, %d", rows[0].Id, rows[1].Id)
	}
	if rows[1].Topic != "message-topic" || rows[1].MessageKey != "9" {
		t.Fatalf("第 2 条字段错误: %+v", rows[1])
	}
}

// MarkPublished 条件更新（status=0→1），锁住"只有 pending 才允许标记已投递"
func TestMarkPublished(t *testing.T) {
	_, mock, m := newTestModel(t)

	mock.ExpectExec("update `event_outbox` set `status` = ? where `id` = ? and `status` = ?").
		WithArgs(StatusPublished, 1, StatusPending).
		WillReturnResult(sqlmock.NewResult(0, 1))

	affected, err := m.MarkPublished(context.Background(), 1)
	if err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	if affected != 1 {
		t.Fatalf("affected = %d, 期望 1", affected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("未满足的 SQL 期望: %v", err)
	}
}
