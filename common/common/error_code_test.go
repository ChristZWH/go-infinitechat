package common

import (
	"errors"
	"strings"
	"testing"
)

// 业务错误码与系统错误码的分界线是 50000：
// 50000 及以上按"系统故障"处理（HTTP 500 + 脱敏），以下按业务错误（HTTP 200 + 业务码）。
// 注意 90xxx / 91xxx 是 WebSocket / RPC 侧的独立区间，不走 HTTP 错误处理器，不在此断言范围内。
func TestErrorCodeSeverityBoundary(t *testing.T) {
	businessCodes := []ErrorCode{
		ParamsError, NotLoginError, NoAuthError, ForbiddenError, NotFoundError,
		PhoneEmailError, UserNotExists, LoginError, PasswordMismatchError,
	}
	for _, code := range businessCodes {
		if code.Code >= 50000 {
			t.Errorf("%s 的业务码 %d 不应 >= 50000（会被当成系统故障返回 500）", code.Message, code.Code)
		}
	}

	systemCodes := []ErrorCode{
		SystemError, OperationError, RedisError, MysqlError,
		SystemBusy, MarshalFailedError, PushFailedError,
	}
	for _, code := range systemCodes {
		if code.Code < 50000 {
			t.Errorf("%s 的系统码 %d 应 >= 50000", code.Message, code.Code)
		}
	}
}

func TestErrorCodeErrorAndUnwrap(t *testing.T) {
	cause := errors.New("sql: 1054 Unknown column")
	wrapped := WrapError(MysqlError, cause)

	if wrapped.Error() != MysqlError.Message {
		t.Errorf("Error() = %q, 期望 %q（对外只暴露脱敏文案）", wrapped.Error(), MysqlError.Message)
	}
	if !errors.Is(wrapped, cause) {
		t.Error("Unwrap 未保留底层错误，errors.Is 匹配不到 Cause")
	}
	if wrapped.Code != MysqlError.Code {
		t.Errorf("Code = %d, 期望 %d", wrapped.Code, MysqlError.Code)
	}

	// 要能通过 errors.As 还原成 ErrorCode —— 统一错误处理器依赖这一点分类
	var got ErrorCode
	if !errors.As(wrapped, &got) {
		t.Fatal("errors.As 未能还原为 ErrorCode")
	}
	if got.Code != MysqlError.Code {
		t.Errorf("还原后的 Code = %d, 期望 %d", got.Code, MysqlError.Code)
	}
}

func TestThrowIf(t *testing.T) {
	assertPanicsWith := func(t *testing.T, want ErrorCode, fn func()) {
		t.Helper()
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("期望 panic，实际没有")
			}
			got, ok := r.(ErrorCode)
			if !ok {
				t.Fatalf("panic 值类型 = %T, 期望 ErrorCode", r)
			}
			if got.Code != want.Code || got.Message != want.Message {
				t.Errorf("panic 错误 = (%d,%q), 期望 (%d,%q)", got.Code, got.Message, want.Code, want.Message)
			}
		}()
		fn()
	}

	t.Run("条件成立时 panic", func(t *testing.T) {
		assertPanicsWith(t, ParamsError, func() { ThrowIf(true, ParamsError) })
	})

	t.Run("条件不成立时不 panic", func(t *testing.T) {
		ThrowIf(false, ParamsError)
	})

	t.Run("ThrowIfWithMsg 覆盖文案", func(t *testing.T) {
		want := ErrorCode{Code: SystemError.Code, Message: "查询群成员数量失败"}
		assertPanicsWith(t, want, func() {
			ThrowIfWithMsg(true, SystemError, "查询群成员数量失败")
		})
	})

	t.Run("Throw 携带 Cause", func(t *testing.T) {
		cause := errors.New("redis: connection refused")
		defer func() {
			r := recover()
			got, ok := r.(ErrorCode)
			if !ok {
				t.Fatalf("panic 值类型 = %T", r)
			}
			if got.Cause == nil || !strings.Contains(got.Cause.Error(), "connection refused") {
				t.Errorf("Cause 未保留底层错误: %v", got.Cause)
			}
		}()
		Throw(SystemError, cause)
	})
}
