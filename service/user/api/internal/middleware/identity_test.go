package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-infinitechat/common/identity"
)

// 这个中间件只解析不拒绝，所以断言点有两处：ctx 里是否拿到身份、请求是否照常往下走。
func TestIdentityMiddlewareParsesHeader(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantUserId int64
		wantOk     bool
	}{
		{"正常数字", "12345", 12345, true},
		{"带空格", "  678  ", 678, true},
		{"没有头", "", 0, false},
		{"非数字", "abc", 0, false},
		{"零", "0", 0, false},
		{"负数", "-5", 0, false},
		{"小数", "1.5", 0, false},
		{"超长数字", "99999999999999999999", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotId int64
			var gotOk bool
			called := false

			next := func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotId, gotOk = identity.UserId(r.Context())
			}

			req := httptest.NewRequest(http.MethodGet, "/api/user/users/1", nil)
			if tt.header != "" {
				req.Header.Set(identity.HeaderUserId, tt.header)
			}
			IdentityMiddleware(next)(httptest.NewRecorder(), req)

			if !called {
				t.Fatal("中间件必须放行到下一个 handler")
			}
			if gotOk != tt.wantOk {
				t.Fatalf("拿到身份 = %v, 期望 %v", gotOk, tt.wantOk)
			}
			if gotOk && gotId != tt.wantUserId {
				t.Fatalf("userId = %d, 期望 %d", gotId, tt.wantUserId)
			}
		})
	}
}

// 不能因为解析失败就中断请求：白名单接口（登录/注册）本来就没有 X-User-Id
func TestIdentityMiddlewareNeverBlocks(t *testing.T) {
	called := false
	next := func(w http.ResponseWriter, r *http.Request) { called = true }

	req := httptest.NewRequest(http.MethodPost, "/api/user/login", nil)
	req.Header.Set(identity.HeaderUserId, "not-a-number")

	rec := httptest.NewRecorder()
	IdentityMiddleware(next)(rec, req)

	if !called {
		t.Fatal("非法身份头不应阻断请求")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("响应码 = %d，中间件不应写响应", rec.Code)
	}
}
