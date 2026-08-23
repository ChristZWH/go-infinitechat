// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package contact

import (
	"go-infinitechat/common/common"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"go-infinitechat/service/user/api/internal/logic/contact"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
)

func ModifyFriendApplicationStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ModifyFriendApplyStatusRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, common.ParamsError)
			return
		}

		l := contact.NewModifyFriendApplicationStatusLogic(r.Context(), svcCtx)
		err := l.ModifyFriendApplicationStatus(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.Ok(w)
		}
	}
}
