// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package contact

import (
	"context"

	dto2 "go-infinitechat/common/model/dto"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/dto"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetApplyListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetApplyListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetApplyListLogic {
	return &GetApplyListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 获取好友申请列表
func (l *GetApplyListLogic) GetApplyList(req *types.FriendApplyListRequest) (resp *dto2.PageResponse[dto.ApplyFriendDTO], err error) {
	return l.svcCtx.ApplyFriendServer.GetReceivedRequestsWithUserInfo(l.ctx, req.UserId, int32(req.PageNum), int32(req.PageSize)), nil
}
