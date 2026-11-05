package user_session

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func newTestModel(t *testing.T) (sqlmock.Sqlmock, *customUserSessionModel) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	conn := sqlx.NewSqlConnFromDB(db)
	m := &customUserSessionModel{
		defaultUserSessionModel: &defaultUserSessionModel{
			CachedConn: sqlc.NewConnWithCache(conn, nil),
			table:      "`user_session`",
		},
	}
	return mock, m
}

// FindMemberIdsBySessionId 曾经把条件写成 `where user_id = ?` 而参数是 sessionId，
// 导致群成员列表恒空（群消息只推给发送者）。这个用例锁住条件必须是 session_id。
func TestFindMemberIdsBySessionId(t *testing.T) {
	t.Run("返回群成员列表", func(t *testing.T) {
		mock, m := newTestModel(t)

		mock.ExpectQuery("select `user_id` from `user_session` where `session_id` = ? and `status` = 0").
			WithArgs(9001).
			WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(10).AddRow(20).AddRow(30))

		ids, err := m.FindMemberIdsBySessionId(context.Background(), 9001)
		if err != nil {
			t.Fatalf("非预期错误: %v", err)
		}
		if len(ids) != 3 || ids[0] != 10 || ids[2] != 30 {
			t.Fatalf("成员列表 = %v, 期望 [10 20 30]", ids)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("未满足的 SQL 期望: %v", err)
		}
	})

	t.Run("查询失败上抛", func(t *testing.T) {
		mock, m := newTestModel(t)

		mock.ExpectQuery("select `user_id` from `user_session` where `session_id` = ? and `status` = 0").
			WithArgs(9001).
			WillReturnError(errors.New("db down"))

		_, err := m.FindMemberIdsBySessionId(context.Background(), 9001)
		if err == nil {
			t.Fatal("查询失败应上抛")
		}
	})
}
