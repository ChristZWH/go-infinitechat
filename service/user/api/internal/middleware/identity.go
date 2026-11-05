package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"go-infinitechat/common/common"
	"go-infinitechat/common/identity"
)

// IdentityMiddleware 把网关注入的 X-User-Id 解析进 request context。
//
// 职责边界：
//   - 只做"解析"，不做"拒绝"。是否必须登录由网关决定（白名单接口本来就没有身份）。
//   - 解析失败（非数字/非正数）只记日志并保持无身份状态，避免一次脏头就让白名单接口 500。
//
// 为什么还要在服务侧解析一遍？因为下游可能被直连（本机调试、内网调用），
// 网关校验不能假设永远生效；同时 logic 层需要一个统一的读取入口。
func IdentityMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if raw := strings.TrimSpace(r.Header.Get(identity.HeaderUserId)); raw != "" {
			if userId, err := strconv.ParseInt(raw, 10, 64); err == nil && userId > 0 {
				r = r.WithContext(identity.WithUserId(r.Context(), userId))
			} else {
				common.Warnf("[Identity] 非法的 %s: %q, path=%s", identity.HeaderUserId, raw, r.URL.Path)
			}
		}
		next(w, r)
	}
}
