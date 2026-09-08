// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package group

import (
	"context"

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
	return l.svcCtx.GroupService.GetGroupMenberCount(l.ctx, req.SessionId), nil
}
