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

type MarkAsReadBatchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkAsReadBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkAsReadBatchLogic {
	return &MarkAsReadBatchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MarkAsReadBatchLogic) MarkAsReadBatch(req *types.MarkAsReadBatchRequest) (resp int32, err error) {
	common.ThrowIfWithMsg(req.UserId <= 0, common.ParamsError, "用户ID无效")
	common.ThrowIfWithMsg(len(req.NotificationIds) == 0, common.ParamsError, "通知ID不能为空")

	affected, err := l.svcCtx.SystemNotificationModel.MarkAsReadBatch(l.ctx, req.UserId, req.NotificationIds)
	if err != nil {
		return 0, common.WrapError(common.MysqlError, err)
	}
	return int32(affected), nil
}
