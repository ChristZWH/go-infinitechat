// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package contact

import (
	"context"

	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ModifyFriendApplicationStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewModifyFriendApplicationStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ModifyFriendApplicationStatusLogic {
	return &ModifyFriendApplicationStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ModifyFriendApplicationStatusLogic) ModifyFriendApplicationStatus(req *types.ModifyFriendApplyStatusRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
