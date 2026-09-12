// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package systemnotification

import (
	"context"
	"errors"

	"go-infinitechat/common/common"
	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"
	"go-infinitechat/service/offline/model/system_notification"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarkAsReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkAsReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkAsReadLogic {
	return &MarkAsReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MarkAsReadLogic) MarkAsRead(req *types.MarkAsReadRequest) (resp bool, err error) {
	common.ThrowIfWithMsg(req.UserId <= 0, common.ParamsError, "用户ID无效")
	common.ThrowIfWithMsg(req.NotificationId <= 0, common.ParamsError, "通知ID无效")

	notification, err := l.svcCtx.SystemNotificationModel.FindOne(l.ctx, req.NotificationId)
	if err != nil {
		if errors.Is(err, system_notification.ErrNotFound) {
			return false, common.NotFoundError
		}
		return false, common.WrapError(common.MysqlError, err)
	}

	//通知必须属于请求的用户
	if notification.ReceiverId != req.UserId {
		return false, common.NoAuthError
	}

	//已读幂等：如果已经是已读状态，直接返回成功
	if notification.IsRead == 1 {
		common.Infof("通知已经是已读状态，跳过，通知ID: %d, 用户ID: %d\n", req.NotificationId, req.UserId)
		return true, nil
	}

	affectedRows, err := l.svcCtx.SystemNotificationModel.MarkAsRead(l.ctx, req.UserId, req.NotificationId)
	if err != nil {
		return false, common.WrapError(common.MysqlError, err)
	}

	common.Infof("标记通知为已读，通知ID: %d, 用户ID: %d, 更新记录数: %d\n", req.NotificationId, req.UserId, affectedRows)
	return affectedRows > 0, nil
}
