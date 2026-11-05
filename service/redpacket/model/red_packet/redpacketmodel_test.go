package red_packet

import (
	"context"
	"errors"
	"testing"

	"go-infinitechat/common/model/txctx"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func newTestModel(t *testing.T) (sqlx.SqlConn, sqlmock.Sqlmock, *customRedPacketModel) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	conn := sqlx.NewSqlConnFromDB(db)
	m := &customRedPacketModel{
		defaultRedPacketModel: &defaultRedPacketModel{
			CachedConn: sqlc.NewConnWithCache(conn, nil),
			table:      "`red_packet`",
		},
	}
	return conn, mock, m
}

// MarkExpiredIfNotCompleted 是"过期退款只退一次"的原子保证：
// 条件更新 status: 0 → 2，RowsAffected==1 才继续退款。
// 并发/重投时第二次 RowsAffected==0，调用方据此跳过退款。
func TestMarkExpiredIfNotCompleted(t *testing.T) {
	const expiredStatus, notCompletedStatus = 2, 0

	t.Run("抢到状态迁移", func(t *testing.T) {
		conn, mock, m := newTestModel(t)

		mock.ExpectBegin()
		mock.ExpectExec("update `red_packet` set `status` = ?, `updated_time` = now() where `red_packet_id` = ? and `status` = ?").
			WithArgs(expiredStatus, 1, notCompletedStatus).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := txctx.WithTransaction(context.Background(), conn, func(ctx context.Context) error {
			affected, err := m.MarkExpiredIfNotCompleted(ctx, 1, expiredStatus, notCompletedStatus)
			if err != nil {
				return err
			}
			if affected != 1 {
				t.Fatalf("affected = %d, 期望 1", affected)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("WithTransaction: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("未满足的 SQL 期望: %v", err)
		}
	})

	t.Run("已被其它消费者处理(affected=0)", func(t *testing.T) {
		conn, mock, m := newTestModel(t)

		mock.ExpectBegin()
		mock.ExpectExec("update `red_packet` set `status` = ?, `updated_time` = now() where `red_packet_id` = ? and `status` = ?").
			WithArgs(expiredStatus, 1, notCompletedStatus).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		err := txctx.WithTransaction(context.Background(), conn, func(ctx context.Context) error {
			affected, err := m.MarkExpiredIfNotCompleted(ctx, 1, expiredStatus, notCompletedStatus)
			if err != nil {
				return err
			}
			if affected != 0 {
				t.Fatalf("affected = %d, 期望 0（已被处理，不得重复退款）", affected)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("WithTransaction: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("未满足的 SQL 期望: %v", err)
		}
	})

	t.Run("更新失败回滚", func(t *testing.T) {
		conn, mock, m := newTestModel(t)

		dbErr := errors.New("db down")
		mock.ExpectBegin()
		mock.ExpectExec("update `red_packet` set `status` = ?, `updated_time` = now() where `red_packet_id` = ? and `status` = ?").
			WithArgs(expiredStatus, 1, notCompletedStatus).
			WillReturnError(dbErr)
		mock.ExpectRollback()

		err := txctx.WithTransaction(context.Background(), conn, func(ctx context.Context) error {
			_, err := m.MarkExpiredIfNotCompleted(ctx, 1, expiredStatus, notCompletedStatus)
			return err
		})
		if err == nil {
			t.Fatal("更新失败应上抛")
		}
	})
}
