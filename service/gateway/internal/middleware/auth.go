package middleware

import (
	"encoding/json"
	"go-infinitechat/common/common"
	"go-infinitechat/service/gateway/internal/config"
	"net/http"
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

		// 4. 将 UserId 注入请求头，传递给下游服务
		r.Header.Set("X-User-Id", userIdStr)

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
