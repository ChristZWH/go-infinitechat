package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-infinitechat/service/gateway/internal/config"
)

func newAuthMiddleware(whiteList ...string) *AuthMiddleWare {
	return NewAuthMiddleware(nil, config.AuthConfig{WhiteList: whiteList})
}

// 白名单曾经和 handler 实际注册的路径漂移（loginPassword/loginCode/register），
// 导致登录注册全被 401 拦掉。这个用例锁住"配置里的路径必须真的能免鉴权"。
func TestWhiteListExactMatch(t *testing.T) {
	m := newAuthMiddleware("/api/user/login", "/api/user/users", "/api/user/sendCaptcha")

	allowed := []string{"/api/user/login", "/api/user/users", "/api/user/sendCaptcha"}
	for _, path := range allowed {
		if !m.isWhiteListed(path) {
			t.Errorf("白名单路径 %s 未命中", path)
		}
	}

	denied := []string{"/api/user/logout", "/api/user/loginPassword", "/api/contact/users/1/friend", ""}
	for _, path := range denied {
		if m.isWhiteListed(path) {
			t.Errorf("路径 %s 不应命中白名单", path)
		}
	}
}

func TestWhiteListRequiresExactPath(t *testing.T) {
	m := newAuthMiddleware("/api/user/login")

	// 带查询串或尾部斜杠时路径不同，当前实现是精确匹配，不会放行
	if m.isWhiteListed("/api/user/login/") {
		t.Error("尾部斜杠不应命中白名单（精确匹配语义）")
	}
	if m.isWhiteListed("/api/user/loginx") {
		t.Error("前缀相同但路径不同不应命中白名单")
	}
}

func TestResponseUnauthorized(t *testing.T) {
	m := newAuthMiddleware()
	rec := httptest.NewRecorder()

	m.responseUnauthorized(rec)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("状态码 = %d, 期望 401", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}

	var body unauthorizedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if body.Code != 401 || body.Message == "" {
		t.Errorf("响应体不正确: %+v", body)
	}
}

// 缺少 Authorization 头时必须直接 401，且不能放行到下游
func TestHandleRejectsMissingToken(t *testing.T) {
	m := newAuthMiddleware("/api/user/login")
	called := false
	next := func(w http.ResponseWriter, r *http.Request) { called = true }

	req := httptest.NewRequest(http.MethodGet, "/api/contact/users/1/friend", nil)
	rec := httptest.NewRecorder()
	m.Handle(next)(rec, req)

	if called {
		t.Fatal("缺少 token 的请求不应进入 next")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("状态码 = %d, 期望 401", rec.Code)
	}
}

// 白名单路径不需要 token
func TestHandleAllowsWhiteListedPath(t *testing.T) {
	m := newAuthMiddleware("/api/user/login")
	called := false
	next := func(w http.ResponseWriter, r *http.Request) { called = true }

	req := httptest.NewRequest(http.MethodPost, "/api/user/login", nil)
	rec := httptest.NewRecorder()
	m.Handle(next)(rec, req)

	if !called {
		t.Fatal("白名单路径应直接放行")
	}
}
