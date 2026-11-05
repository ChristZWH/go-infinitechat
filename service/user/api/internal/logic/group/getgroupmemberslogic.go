// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package group

import (
	"context"

	"go-infinitechat/common/common"
	"go-infinitechat/common/identity"
	dto2 "go-infinitechat/common/model/dto"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/dto"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGroupMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetGroupMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupMembersLogic {
	return &GetGroupMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 获取群聊成员
func (l *GetGroupMembersLogic) GetGroupMembers(req *types.GetGroupMembersRequest) (resp *dto2.PageResponse[dto.GroupMemberDTO], err error) {
	// 会话维度的授权：必须是群成员才能看成员列表
	userId, ok := identity.UserId(l.ctx)
	common.ThrowIf(!ok, common.PleaseLogin)
	l.svcCtx.GroupService.EnsureGroupMember(l.ctx, req.SessionId, userId)
	return l.svcCtx.GroupService.GetGroupMembers(l.ctx, req.SessionId, int32(req.PageNum), int32(req.PageSize)), nil
}
