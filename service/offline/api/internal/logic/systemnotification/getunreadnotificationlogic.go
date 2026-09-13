// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package systemnotification

import (
	"context"

	"go-infinitechat/common/common"
	"go-infinitechat/common/model/dto"
	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"
	"go-infinitechat/service/offline/model/system_notification"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUnreadNotificationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUnreadNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUnreadNotificationLogic {
	return &GetUnreadNotificationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUnreadNotificationLogic) GetUnreadNotification(req *types.GetUnreadNotificationsRequest) (*dto.PageResponse[system_notification.SystemNotification], error) {
	common.ThrowIfWithMsg(req.UserId <= 0, common.ParamsError, "用户ID无效")

	// 先转 int64 再乘，防止 int32 乘法溢出
	offset := int64(req.PageNum-1) * int64(req.PageSize)
	limit := int(req.PageSize)
	notification, err := l.svcCtx.SystemNotificationModel.FindUnreadByReceiverId(l.ctx, req.UserId, int(offset), limit)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "GetUnreadNotification 查询失败", err)

	total, err := l.svcCtx.SystemNotificationModel.CountUnreadByReceiverId(l.ctx, req.UserId)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "GetUnreadNotification 统计失败", err)

	records := make([]system_notification.SystemNotification, 0, len(notification))
	for _, n := range notification {
		records = append(records, *n)
	}

	return dto.NewPageResponse(records, int(total), int(req.PageNum), limit), nil
}
