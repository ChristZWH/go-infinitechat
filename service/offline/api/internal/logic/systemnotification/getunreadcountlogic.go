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

type GetUnreadCountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUnreadCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUnreadCountLogic {
	return &GetUnreadCountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUnreadCountLogic) GetUnreadCount(req *types.GetUnreadCountRequest) (map[string]int64, error) {
	common.ThrowIfWithMsg(req.UserId <= 0, common.ParamsError, "用户ID无效")

	resp, err := l.svcCtx.SystemNotificationModel.CountUnreadByReceiverId(l.ctx, req.UserId)
	if err != nil {
		return nil, common.WrapError(common.MysqlError, err)
	}
	return map[string]int64{"count": resp}, nil
}
