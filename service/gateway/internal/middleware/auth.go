package middleware

import (
	"encoding/json"
	"go-infinitechat/common/common"
	"go-infinitechat/common/identity"
	"go-infinitechat/service/gateway/internal/config"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	AccessTokenKeyPrefix = "access_token:"
)

type unauthorizedResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type AuthMiddleWare struct {
	rds       *redis.Redis
	whiteList map[string]struct{}
}

func NewAuthMiddleware(rds *redis.Redis, authConf config.AuthConfig) *AuthMiddleWare {
	wl := make(map[string]struct{}, len(authConf.WhiteList))
	for _, path := range authConf.WhiteList {
		wl[path] = struct{}{}
	}
	return &AuthMiddleWare{
		rds:       rds,
		whiteList: wl,
	}
}

func (m *AuthMiddleWare) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// 1. 白名单路径直接放行
		if m.isWhiteListed(path) {
			next(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		if token == "" {
			common.Warnf("[Gateway] 缺少Authorization Header, path: %s\n", path)
			m.responseUnauthorized(w)
			return
		}

		// 兼容 "Bearer <token>" 格式
		token = strings.TrimPrefix(token, "Bearer ")
		token = strings.TrimSpace(token)

		if token == "" {
			common.Warnf("[Gateway] Token为空, path: %s\n", path)
			m.responseUnauthorized(w)
			return
		}

		userIdStr, err := m.rds.Get(AccessTokenKeyPrefix + token)
		if err != nil || userIdStr == "" {
			common.Warnf("[Gateway] Token不合法, token: %s, err: %v\n", token, err)
			m.responseUnauthorized(w)
			return
		}

		common.Infof("[Gateway] Token合法, userId: %s, path: %s\n", userIdStr, path)

		// 4. 身份一致性校验：/users/{id} 里的 id 必须是 token 的主人。
		//    否则 A 登录后带上 B 的 id 就能调他人接口（此前 30+ 个接口都存在这个越权）。
		if isPathUserIdMismatch(path, userIdStr) {
			common.Warnf("[Gateway] 越权访问被拒绝: token userId=%s, path=%s", userIdStr, path)
			m.responseForbidden(w)
			return
		}

		// 5. 先删掉客户端可能自带的身份头（防伪造），再写入可信身份
		r.Header.Del(identity.HeaderUserId)
		r.Header.Set(identity.HeaderUserId, userIdStr)

		next(w, r)
	}
}

// 检查路径是否在白名单中
func (m *AuthMiddleWare) isWhiteListed(path string) bool {
	// 精确匹配
	if _, ok := m.whiteList[path]; ok {
		return true
	}
	return false
}

// 返回 401 JSON 响应: {"code":401,"message":"未授权的访问，请提供有效的Token"}
func (m *AuthMiddleWare) responseUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)

	resp := unauthorizedResponse{
		Code:    401,
		Message: "未授权的访问，请提供有效的Token",
	}
	json.NewEncoder(w).Encode(resp)
}

// 返回 403：已登录，但试图操作他人的数据
func (m *AuthMiddleWare) responseForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)

	resp := unauthorizedResponse{
		Code:    40300,
		Message: "无权访问他人的数据",
	}
	json.NewEncoder(w).Encode(resp)
}

// userPathSegment 匹配 /users/{数字}，捕获其中的 id。
//
// 约定的路径语义：路径里第一个 /users/{id} 永远是"当前操作者"，
// 其他用户以独立参数出现，例如 /users/:userId/friend/:receiveUserId 的 receiveUserId。
var userPathSegment = regexp.MustCompile(`/users/(\d+)(?:/|$)`)

// isPathUserIdMismatch 判断路径里的操作者 id 是否与 token 主人一致。
// 没有 /users/{id} 的路径（例如 /api/group/:sessionId/members）返回 false，
// 这类接口的归属校验属于会话成员维度，需要各服务自己判断。
func isPathUserIdMismatch(path, tokenUserId string) bool {
	matches := userPathSegment.FindStringSubmatch(path)
	if len(matches) < 2 {
		return false
	}

	pathUserId, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil || pathUserId <= 0 {
		// 路径里不是合法 id，交给下游的参数校验去报错，避免这里误判成越权
		return false
	}

	authUserId, err := strconv.ParseInt(strings.TrimSpace(tokenUserId), 10, 64)
	if err != nil || authUserId <= 0 {
		// token 对应的 userId 不可解析：环境异常，按拒绝处理
		return true
	}

	return pathUserId != authUserId
}
