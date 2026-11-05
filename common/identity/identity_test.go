package identity

import (
	"context"
	"testing"
)

func TestWithUserIdRoundTrip(t *testing.T) {
	ctx := WithUserId(context.Background(), 12345)

	got, ok := UserId(ctx)
	if !ok {
		t.Fatal("写入后应能读出用户 ID")
	}
	if got != 12345 {
		t.Fatalf("UserId = %d, 期望 12345", got)
	}
}

func TestUserIdMissing(t *testing.T) {
	for name, ctx := range map[string]context.Context{
		"背景 context": context.Background(),
		"nil context":  nil,
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := UserId(ctx); ok {
				t.Fatal("没有身份时 ok 应为 false")
			}
		})
	}
}

func TestWithUserIdRejectsInvalid(t *testing.T) {
	// 0 与负数不是合法用户 ID，写进去等于没写，避免"0 被当成合法身份"绕过校验
	for _, id := range []int64{0, -1, -100} {
		ctx := WithUserId(context.Background(), id)
		if _, ok := UserId(ctx); ok {
			t.Fatalf("非法 ID %d 不应被写入 ctx", id)
		}
	}
}

func TestUserIdDoesNotLeakToChild(t *testing.T) {
	// 身份只能从父 ctx 往下传，不能反向影响
	parent := WithUserId(context.Background(), 1)
	other := WithUserId(context.Background(), 2)

	if got, _ := UserId(parent); got != 1 {
		t.Fatalf("parent = %d, 期望 1", got)
	}
	if got, _ := UserId(other); got != 2 {
		t.Fatalf("other = %d, 期望 2", got)
	}
}

func TestUserIdInChildContext(t *testing.T) {
	parent := WithUserId(context.Background(), 42)
	child, cancel := context.WithCancel(parent)
	defer cancel()

	got, ok := UserId(child)
	if !ok || got != 42 {
		t.Fatalf("子 context 应继承身份, got=%d ok=%v", got, ok)
	}
}

func TestUserIdOverwrite(t *testing.T) {
	ctx := WithUserId(context.Background(), 1)
	ctx = WithUserId(ctx, 2)

	if got, _ := UserId(ctx); got != 2 {
		t.Fatalf("后写入的身份应覆盖之前的值, got=%d", got)
	}
}
