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

type UserInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserInfoLogic {
	return &UserInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 获取用户基本信息
func (l *UserInfoLogic) UserInfo(req *types.UserIdPathRequest) (resp *types.UserInfoResponse, err error) {
	userInfo, err := l.svcCtx.UserModel.FindOne(l.ctx, req.UserId)
	// 区分"用户不存在"和系统错误：DB 故障不能返回 404
	common.ThrowIf(err != nil && !errors.Is(err, sqlx.ErrNotFound), common.SystemError, err)
	common.ThrowIf(userInfo == nil, common.NotFoundError)

	return &types.UserInfoResponse{
		UserId:   userInfo.UserId,
		Nickname: userInfo.Nickname.String,
		Avatar:   userInfo.Avatar,
	}, nil
}
