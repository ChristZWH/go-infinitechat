package txctx

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type txKey struct{}

// 从 context 中取出事务 session，没有则返回 nil
func GetSession(ctx context.Context) sqlx.Session {
	s, _ := ctx.Value(txKey{}).(sqlx.Session)
	return s
}

// 开启事务，把 session 塞进 context。如果 ctx 中已经存在事务 session，则复用外层事务，不再开新事务
func WithTransaction(ctx context.Context, conn sqlx.SqlConn, fn func(ctx context.Context) error) error {
	if GetSession(ctx) != nil {
		return fn(ctx)
	}
	return conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		txCtx := context.WithValue(ctx, txKey{}, session)
		return safeRun(func() error {
			return fn(txCtx)
		})
	})
}

// 把 panic 转为 error，让事务能正确 rollback
func safeRun(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if bizerr, ok := r.(error); ok {
				err = bizerr
			} else {
				err = fmt.Errorf("panic: %v", r)
			}
		}
	}()
	return fn()
}
