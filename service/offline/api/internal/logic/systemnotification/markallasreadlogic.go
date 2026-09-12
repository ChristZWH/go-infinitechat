// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package systemnotification

import (
	"context"

	"go-infinitechat/common/common"
	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarkAllAsReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkAllAsReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkAllAsReadLogic {
	return &MarkAllAsReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MarkAllAsReadLogic) MarkAllAsRead(req *types.MarkAllAsReadRequest) (resp int, err error) {
	common.ThrowIfWithMsg(req.UserId <= 0, common.ParamsError, "用户ID无效")

	affected, err := l.svcCtx.SystemNotificationModel.MarkAllAsRead(l.ctx, req.UserId)
	if err != nil {
		return 0, common.WrapError(common.MysqlError, err)
	}
	return int(affected), nil
}
