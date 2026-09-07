// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"
	"errors"

	"go-infinitechat/common/common"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type UpdateAvatarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAvatarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAvatarLogic {
	return &UpdateAvatarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAvatarLogic) UpdateAvatar(req *types.UpdateAvatarRequest) (bool, error) {
	user, err := l.svcCtx.UserService.GetUserById(req.UserId)
	// 区分"用户不存在"和系统错误：DB 故障不能返回 404
	common.ThrowIf(err != nil && errors.Is(err, sqlx.ErrNotFound), common.SystemError, err)
	common.ThrowIf(user == nil, common.NotFoundError)

	l.svcCtx.UserService.UpdateAvatar(l.ctx, req.UserId, req.URL)
	return true, nil
}
