// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package group

import (
	"context"

	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/dto"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGroupLogic {
	return &CreateGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 创建群聊
func (l *CreateGroupLogic) CreateGroup(req *types.CreateGroupRequest) (resp *dto.CreateGroupResponse, err error) {
	return l.svcCtx.GroupService.CreateGroup(l.ctx, req), nil
}
