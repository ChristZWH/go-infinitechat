package middleware

import (
	"go-infinitechat/service/gateway/internal/config"
	"net/http"
	"strconv"
	"strings"
)

type CorsMiddleware struct {
	conf config.CorsConfig
}

func NewCorsMiddleware(conf config.CorsConfig) *CorsMiddleware {
	return &CorsMiddleware{conf: conf}
}

func (m *CorsMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// 检查 Origin 是否在允许列表中
		if m.isOriginAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.conf.AllowedMethods, ", "))
		w.Header().Set("Access-Control-Allow-Headers", m.conf.AllowedHeaders)
		w.Header().Set("Access-Control-Allow-Credentials", strconv.FormatBool(m.conf.AllowCredentials))
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(m.conf.MaxAge))

		// OPTIONS 预检请求直接返回
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

func (m *CorsMiddleware) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	for _, allowed := range m.conf.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}
