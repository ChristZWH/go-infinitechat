// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package group

import (
	"context"

	dto2 "go-infinitechat/common/model/dto"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/dto"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserGroupsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserGroupsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserGroupsLogic {
	return &GetUserGroupsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 获取用户加入的所有群聊
func (l *GetUserGroupsLogic) GetUserGroups(req *types.GetUserGroupsRequest) (resp *dto2.PageResponse[dto.UserGroupDTO], err error) {
	return l.svcCtx.GroupService.GetUserGroups(l.ctx, req.UserId, int32(req.PageNum), int32(req.PageSize)), nil
}
