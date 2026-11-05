// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package group

import (
	"context"

	"go-infinitechat/common/common"
	"go-infinitechat/common/identity"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGroupMemberCountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetGroupMemberCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupMemberCountLogic {
	return &GetGroupMemberCountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 获取群聊人数
func (l *GetGroupMemberCountLogic) GetGroupMemberCount(req *types.SessionIdPathRequest) (resp int, err error) {
	// 会话维度的授权：必须是群成员才能看人数
	userId, ok := identity.UserId(l.ctx)
	common.ThrowIf(!ok, common.PleaseLogin)
	l.svcCtx.GroupService.EnsureGroupMember(l.ctx, req.SessionId, userId)
	return l.svcCtx.GroupService.GetGroupMemberCount(l.ctx, req.SessionId), nil
}
