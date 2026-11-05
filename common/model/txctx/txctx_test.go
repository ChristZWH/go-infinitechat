package txctx

import (
	"context"
	"errors"
	"testing"

	"go-infinitechat/common/common"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type fakeSession struct{ sqlx.Session }

type fakeConn struct {
	sqlx.SqlConn
	calls int
}

func (f *fakeConn) TransactCtx(_ context.Context, fn func(context.Context, sqlx.Session) error) error {
	f.calls++
	return fn(context.Background(), &fakeSession{})
}

func TestWithTransactionInjectsSession(t *testing.T) {
	conn := &fakeConn{}
	err := WithTransaction(context.Background(), conn, func(ctx context.Context) error {
		if GetSession(ctx) == nil {
			t.Fatal("事务内应能拿到 session")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction: %v", err)
	}
	if conn.calls != 1 {
		t.Fatalf("TransactCtx 调用 %d 次, 期望 1", conn.calls)
	}
}

// 嵌套事务必须复用外层 session，而不是再开一个数据库事务
func TestWithTransactionReusesOuterSession(t *testing.T) {
	conn := &fakeConn{}
	innerRuns := 0

	err := WithTransaction(context.Background(), conn, func(ctx context.Context) error {
		return WithTransaction(ctx, conn, func(innerCtx context.Context) error {
			innerRuns++
			if GetSession(innerCtx) == nil {
				t.Fatal("内层应复用外层 session")
			}
			return nil
		})
	})
	if err != nil {
		t.Fatalf("WithTransaction: %v", err)
	}
	if conn.calls != 1 {
		t.Fatalf("TransactCtx 调用 %d 次, 期望 1（内层复用外层，不再 BEGIN）", conn.calls)
	}
	if innerRuns != 1 {
		t.Fatalf("内层 fn 运行 %d 次, 期望 1", innerRuns)
	}
}

// 事务里的 Throw(panic) 必须被转成 error，让上层能感知并回滚
func TestWithTransactionConvertsPanicToError(t *testing.T) {
	conn := &fakeConn{}
	err := WithTransaction(context.Background(), conn, func(ctx context.Context) error {
		common.Throw(common.SystemError)
		return nil
	})
	if err == nil {
		t.Fatal("panic 应被转为 error 返回")
	}

	var ec common.ErrorCode
	if !errors.As(err, &ec) {
		t.Fatalf("错误类型 = %T, 期望 ErrorCode", err)
	}
	if ec.Code != common.SystemError.Code {
		t.Fatalf("错误码 = %d, 期望 %d", ec.Code, common.SystemError.Code)
	}
}

// 非 error 的 panic 也应被安全包装成 error
func TestWithTransactionConvertsRawPanic(t *testing.T) {
	conn := &fakeConn{}
	err := WithTransaction(context.Background(), conn, func(ctx context.Context) error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("裸 panic 应被转为 error 返回")
	}
}
