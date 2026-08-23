package middleware

import (
	"go-infinitechat/common/common"
	"go-infinitechat/common/utils"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func RecoverMiddleWare(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				if bizErr, ok := err.(common.ErrorCode); ok {
					// 系统级错误 走 ErrorCtx，让user.go 的 SetErrorHandlerCtx 同一处理
					if bizErr.Code >= 50000 {
						httpx.ErrorCtx(r.Context(), w, bizErr)
						return
					}
					// 业务错误，正常返回
					httpx.OkJsonCtx(r.Context(), w, utils.Fail(bizErr))
					return
				}

				common.Errorf("服务器内部问题", err)
				httpx.ErrorCtx(r.Context(), w, common.SystemError)
			}
		}()
		next(w, r)
	}
}
