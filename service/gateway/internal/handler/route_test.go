package handler

import (
	"testing"
)

func newTestRouter(prefixes ...[]string) *ProxyRouter {
	pr := &ProxyRouter{}
	for i, p := range prefixes {
		pr.routes = append(pr.routes, &RouteEntry{
			Id:       string(rune('A' + i)),
			Prefixes: p,
		})
	}
	return pr
}

func TestFindRoutesMatchesFirstPrefix(t *testing.T) {
	pr := newTestRouter(
		[]string{"/api/user", "/api/contact", "/api/group"},
		[]string{"/api/offline"},
		[]string{"/api/chat/redPacket"},
	)

	tests := []struct {
		path    string
		wantId  string
		wantNil bool
	}{
		{"/api/user/login", "A", false},
		{"/api/contact/users/1/friend", "A", false},
		{"/api/group/create", "A", false},
		{"/api/offline/history", "B", false},
		{"/api/chat/redPacket/send", "C", false},
		{"/api/unknown", "", true},
		{"/", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			route := pr.FindRoutes(tt.path)
			if tt.wantNil {
				if route != nil {
					t.Fatalf("路径 %s 不应匹配到路由，实际匹配 %s", tt.path, route.Id)
				}
				return
			}
			if route == nil {
				t.Fatalf("路径 %s 未匹配到路由", tt.path)
			}
			if route.Id != tt.wantId {
				t.Errorf("路径 %s 命中路由 %s, 期望 %s", tt.path, route.Id, tt.wantId)
			}
		})
	}
}

func TestFindRoutesIsPrefixNotSuffixMatch(t *testing.T) {
	pr := newTestRouter([]string{"/api/user"})

	// 前缀匹配：/api/users 也会命中 /api/user —— 这是当前语义（配置顺序决定优先级）
	if route := pr.FindRoutes("/api/users"); route == nil {
		t.Error("/api/users 应命中前缀 /api/user")
	}
	// 但 /x/api/user 不应命中
	if route := pr.FindRoutes("/x/api/user"); route != nil {
		t.Error("/x/api/user 不应命中前缀 /api/user")
	}
}

func TestFindRoutesEmpty(t *testing.T) {
	pr := newTestRouter()
	if route := pr.FindRoutes("/api/user"); route != nil {
		t.Fatalf("没有路由时应返回 nil, 实际 %s", route.Id)
	}
}
