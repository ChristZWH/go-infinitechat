// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package systemnotification

import (
	"context"

	"go-infinitechat/service/offline/api/internal/svc"
	"go-infinitechat/service/offline/api/internal/types"

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

func (l *GetUnreadNotificationLogic) GetUnreadNotification(req *types.GetUnreadNotificationsRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
