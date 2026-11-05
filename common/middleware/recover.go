package middleware

import (
	"net/http"
	"runtime/debug"

	"go-infinitechat/common/common"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func RecoverMiddleWare(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				bizErr, ok := err.(common.ErrorCode)
				if !ok {
					// 未知 panic：必须记堆栈（线上排查靠它），对外脱敏
					common.Errorf("未知panic: %v\n%s", err, debug.Stack())
					httpx.ErrorCtx(r.Context(), w, common.SystemError)
					return
				}
				if bizErr.Code >= 50000 {
					// 系统级错误记日志；业务错误是 Throw 抛出的正常流程，不记避免刷屏
					// 注意：ErrorCode.Error() 只返回 Message，真正的底层原因在 Cause 里，
					// 不显式打出来就永远看不到（例如 SQL 的 1054 Unknown column）
					if bizErr.Cause != nil {
						common.Errorf("系统错误 %d: %s, 原因: %v", bizErr.Code, bizErr.Message, bizErr.Cause)
					} else {
						common.Errorf("系统错误 %d: %s", bizErr.Code, bizErr.Message)
					}
				}
				// 全部交给 SetErrorHandlerCtx 统一分类、统一格式
				// 注意：不能走 OkJsonCtx，否则 SetOkHandler 会把错误体再包一层 Success
				httpx.ErrorCtx(r.Context(), w, bizErr)
			}
		}()
		next(w, r)
	}
}
	// ============ 1. Redis ============
