package apply_friend

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func newTestModel(t *testing.T) (sqlmock.Sqlmock, *customApplyFriendModel) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	conn := sqlx.NewSqlConnFromDB(db)
	m := &customApplyFriendModel{
		defaultApplyFriendModel: &defaultApplyFriendModel{
			CachedConn: sqlc.NewConnWithCache(conn, nil),
			table:      "`apply_friend`",
		},
	}
	return mock, m
}

// FindPageByUserIdWithPagination 曾经把 `receiver_id = ?` 整个包进反引号，
// 且分页查询失败被吞成空列表。这个用例锁住：正常分页、计数错误上抛、分页错误上抛。
func TestFindPageByUserIdWithPagination(t *testing.T) {
	t.Run("正常返回总数与列表", func(t *testing.T) {
		mock, m := newTestModel(t)

		mock.ExpectQuery("select count\\(\\*\\) from `apply_friend` where `sender_id` = \\? or `receiver_id` = \\?").
			WithArgs(1, 1).
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(3))

		mock.ExpectQuery("select .* from `apply_friend` where `sender_id` = \\? or `receiver_id` = \\? order by `updated_time` desc limit \\? offset \\?").
			WithArgs(1, 1, 20, 0).
			WillReturnRows(sqlmock.NewRows([]string{
				"apply_friend_id", "sender_id", "receiver_id", "message", "status", "created_time", "updated_time",
			}).
				AddRow(10, 1, 2, "hi", 0, time.Now(), time.Now()).
				AddRow(11, 1, 3, "hey", 0, time.Now(), time.Now()).
				AddRow(12, 1, 4, "yo", 0, time.Now(), time.Now()))

		rows, total, err := m.FindPageByUserIdWithPagination(context.Background(), 1, 1, 20)
		if err != nil {
			t.Fatalf("非预期错误: %v", err)
		}
		if total != 3 {
			t.Fatalf("total = %d, 期望 3", total)
		}
		if len(rows) != 3 {
			t.Fatalf("列表长度 = %d, 期望 3", len(rows))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("未满足的 SQL 期望: %v", err)
		}
	})

	t.Run("计数失败上抛", func(t *testing.T) {
		mock, m := newTestModel(t)

		mock.ExpectQuery("select count\\(\\*\\) from `apply_friend` where `sender_id` = \\? or `receiver_id` = \\?").
			WithArgs(1, 1).
			WillReturnError(errors.New("db down"))

		_, _, err := m.FindPageByUserIdWithPagination(context.Background(), 1, 1, 20)
		if err == nil {
			t.Fatal("计数失败应上抛，实际被吞掉")
		}
	})

	t.Run("分页失败上抛（曾经被吞成空页）", func(t *testing.T) {
		mock, m := newTestModel(t)

		mock.ExpectQuery("select count\\(\\*\\) from `apply_friend` where `sender_id` = \\? or `receiver_id` = \\?").
			WithArgs(1, 1).
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))

		mock.ExpectQuery("select .* from `apply_friend` where `sender_id` = \\? or `receiver_id` = \\? order by `updated_time` desc limit \\? offset \\?").
			WithArgs(1, 1, 20, 0).
			WillReturnError(errors.New("db down"))

		_, _, err := m.FindPageByUserIdWithPagination(context.Background(), 1, 1, 20)
		if err == nil {
			t.Fatal("分页失败应上抛，实际被吞掉")
		}
	})
}
