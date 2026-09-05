// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"
	"errors"
	"strconv"

	"go-infinitechat/common/common"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginRequest) (resp *types.LoginResponse, err error) {
	user, err := l.svcCtx.UserService.GetUser(req.Account)
	if err != nil {
		// 区别没找到还是系统错误
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.WrapError(common.UserNotExists, err)
		}
		return nil, common.WrapError(common.SystemError, err)
	}
	if user == nil {
		return nil, common.UserNotExists
	}

	// 验证密码是否正确
	encryptedPassword := l.svcCtx.UserService.EncryptPassword(req.Password)
	if user.Password.String != encryptedPassword {
		return nil, common.LoginError
	}

	twoToken := l.svcCtx.UserService.GetTwoToken(user.UserId)

	wsServerUri := l.svcCtx.WsServerLocator.GetWsServerUri(strconv.FormatInt(user.UserId, 10))
	l.svcCtx.Redis.Hset("wsServiceUri", strconv.FormatInt(user.UserId, 10), wsServerUri)

	return &types.LoginResponse{
		UserId: user.UserId,
		// Nickname: user.Nickname.Value(),
		Avatar:       user.Avatar,
		Gender:       user.Gender,
		Description:  user.Description.String,
		Token:        twoToken.AccessToken,
		RefreshToken: twoToken.RefreshToken,
		WeServiceURL: wsServerUri,
	}, nil
}
