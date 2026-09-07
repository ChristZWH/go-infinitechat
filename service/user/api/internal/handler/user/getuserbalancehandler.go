// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"go-infinitechat/common/common"
	"net/http"

	"go-infinitechat/service/user/api/internal/logic/user"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetUserBalanceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UserIdPathRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, common.ParamsError)
			return
		}

		l := user.NewGetUserBalanceLogic(r.Context(), svcCtx)
		resp, err := l.GetUserBalance(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
