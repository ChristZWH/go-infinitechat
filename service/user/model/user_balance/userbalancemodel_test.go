package user_balance

import (
	"context"
	"errors"
	"testing"

	"go-infinitechat/common/model/txctx"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// newTestModel 用 sqlmock 构造 model。
// 关键：CachedConn 用 nil cache（NewConnWithCache），这样 DeductBalanceTx/AddBalanceTx
// 在事务分支里只走 session.ExecCtx，完全不碰 Redis，测试无需外部依赖。
func newTestModel(t *testing.T) (sqlx.SqlConn, sqlmock.Sqlmock, *customUserBalanceModel) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	conn := sqlx.NewSqlConnFromDB(db)
	m := &customUserBalanceModel{
		defaultUserBalanceModel: &defaultUserBalanceModel{
			CachedConn: sqlc.NewConnWithCache(conn, nil),
			table:      "`user_balance`",
		},
	}
	return conn, mock, m
}

// DeductBalanceTx 曾经把列名拼成 `updateed_time`，一旦执行必报 1054。
// 这个用例锁住"扣款语句能被 MySQL 解析、余额足够时扣减成功"。
func TestDeductBalanceTx(t *testing.T) {
	t.Run("余额充足扣减成功", func(t *testing.T) {
		conn, mock, m := newTestModel(t)

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_balance` SET `balance` = `balance` - ? WHERE `user_id` = ? AND `balance` >= ?").
			WithArgs(10, 1, 10).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := txctx.WithTransaction(context.Background(), conn, func(ctx context.Context) error {
			affected, err := m.DeductBalanceTx(ctx, 1, 10)
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

	t.Run("余额不足 affected=0", func(t *testing.T) {
		conn, mock, m := newTestModel(t)

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_balance` SET `balance` = `balance` - ? WHERE `user_id` = ? AND `balance` >= ?").
			WithArgs(100, 1, 100).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		err := txctx.WithTransaction(context.Background(), conn, func(ctx context.Context) error {
			affected, err := m.DeductBalanceTx(ctx, 1, 100)
			if err != nil {
				return err
			}
			if affected != 0 {
				t.Fatalf("affected = %d, 期望 0（余额不足）", affected)
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

	t.Run("数据库报错回滚并上抛", func(t *testing.T) {
		conn, mock, m := newTestModel(t)

		dbErr := errors.New("Error 1054: Unknown column")
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_balance` SET `balance` = `balance` - ? WHERE `user_id` = ? AND `balance` >= ?").
			WithArgs(10, 1, 10).
			WillReturnError(dbErr)
		mock.ExpectRollback()

		err := txctx.WithTransaction(context.Background(), conn, func(ctx context.Context) error {
			_, err := m.DeductBalanceTx(ctx, 1, 10)
			return err
		})
		if err == nil {
			t.Fatal("期望上抛数据库错误")
		}
		if !errors.Is(err, dbErr) {
			t.Fatalf("错误 = %v, 期望 %v", err, dbErr)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("未满足的 SQL 期望: %v", err)
		}
	})
}

func TestAddBalanceTx(t *testing.T) {
	conn, mock, m := newTestModel(t)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_balance` SET `balance` = `balance` + ? WHERE `user_id` = ? ").
		WithArgs(50, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := txctx.WithTransaction(context.Background(), conn, func(ctx context.Context) error {
		return m.AddBalanceTx(ctx, 1, 50)
	})
	if err != nil {
		t.Fatalf("WithTransaction: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("未满足的 SQL 期望: %v", err)
	}
}
