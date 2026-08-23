// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"go-infinitechat/common/common"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"go-infinitechat/service/user/api/internal/logic/user"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
)

func GetUploadUrlHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetUploadURLRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, common.ParamsError)
			return
		}

		l := user.NewGetUploadUrlLogic(r.Context(), svcCtx)
		resp, err := l.GetUploadUrl(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
