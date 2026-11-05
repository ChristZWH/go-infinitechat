package offlinedata

import (
	"context"
	"errors"
	"testing"

	"go-infinitechat/common/common"
	"go-infinitechat/common/identity"
	"go-infinitechat/service/user/rpc/userrpc"

	"google.golang.org/grpc"
)

// fakeUserRpc 只实现被测代码用到的 GetUserSessionRole，其余方法由内嵌接口兜底
type fakeUserRpc struct {
	userrpc.UserRpc
	roleErr error
}

func (f *fakeUserRpc) GetUserSessionRole(_ context.Context, _ *userrpc.UserSessionRoleReq, _ ...grpc.CallOption) (*userrpc.UserSessionRoleResp, error) {
	if f.roleErr != nil {
		return nil, f.roleErr
	}
	return &userrpc.UserSessionRoleResp{Role: 2}, nil
}

// assertPanicCode 断言 fn 抛出的 panic 是带指定错误码的 ErrorCode
func assertPanicCode(t *testing.T, want common.ErrorCode, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("期望 panic(ErrorCode %d), 实际没有", want.Code)
		}
		got, ok := r.(common.ErrorCode)
		if !ok {
			t.Fatalf("panic 类型 = %T, 期望 ErrorCode", r)
		}
		if got.Code != want.Code {
			t.Fatalf("错误码 = %d, 期望 %d", got.Code, want.Code)
		}
	}()
	fn()
}

func TestEnsureSessionMember(t *testing.T) {
	t.Run("已登录且是成员", func(t *testing.T) {
		ctx := identity.WithUserId(context.Background(), 1001)
		got := ensureSessionMember(ctx, &fakeUserRpc{}, 55)
		if got != 1001 {
			t.Fatalf("返回 userId = %d, 期望 1001", got)
		}
	})

	t.Run("未登录", func(t *testing.T) {
		assertPanicCode(t, common.PleaseLogin, func() {
			ensureSessionMember(context.Background(), &fakeUserRpc{}, 55)
		})
	})

	t.Run("不是会话成员", func(t *testing.T) {
		ctx := identity.WithUserId(context.Background(), 1001)
		rpc := &fakeUserRpc{roleErr: errors.New("sql: no rows")}
		assertPanicCode(t, common.NoAuthError, func() {
			ensureSessionMember(ctx, rpc, 55)
		})
	})
}
