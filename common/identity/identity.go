// Package identity 承载"当前请求是谁"这个事实。
//
// 设计约定：
//   - 身份只能由网关写入的 HeaderUserId 推导，业务代码不再信任 URL/body 里的 userId
//   - 解析发生在服务入口的中间件里，logic 层通过 UserId(ctx) 读取
//
// 这样做的原因见 docs/roadmap.md 第 1 节：此前所有接口的 userId 都来自路径或请求体，
// 任意登录用户可以冒充他人。把身份收敛到 ctx 之后，越权只可能发生在"没有校验 ctx"的地方，
// 可以用静态检查/测试批量兜住。
package identity

import "context"

// HeaderUserId 网关鉴权通过后写入的身份头。下游服务只读这个头，不读请求体里的 userId。
const HeaderUserId = "X-User-Id"

type ctxKey struct{}

// WithUserId 把已认证的用户 ID 写入 context
func WithUserId(ctx context.Context, userId int64) context.Context {
	if userId <= 0 {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, userId)
}

// UserId 读取当前请求的已认证用户 ID。
// 返回 ok=false 表示这次请求没有经过网关鉴权（例如直连服务、或未带 token 的白名单接口）。
func UserId(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	id, ok := ctx.Value(ctxKey{}).(int64)
	if !ok || id <= 0 {
		return 0, false
	}
	return id, true
}

// MustUserId 用于"必须有身份"的场景：取不到就交给调用方按 401 处理。
// 单独提供这个函数是为了让 logic 层一眼能看出"这里依赖登录态"。
func MustUserId(ctx context.Context) (int64, bool) {
	return UserId(ctx)
}

// OverrideIfPresent 如果 ctx 里有已认证身份，就覆盖 *id。
// 用于 logic 层一行完成"以登录态为准，忽略请求里带的 userId"：
//
//	identity.OverrideIfPresent(l.ctx, &req.UserId)
//
// 没有身份时（例如直连服务调试、白名单接口）保持原值不变。
func OverrideIfPresent(ctx context.Context, id *int64) {
	if id == nil {
		return
	}
	if v, ok := UserId(ctx); ok {
		*id = v
	}
}
