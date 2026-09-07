// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"
	"strconv"

	"go-infinitechat/common/common"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/constants"
	"go-infinitechat/service/user/api/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefreshTokenLogic) RefreshToken(req *types.RefreshTokenRequest) (resp string, err error) {
	refreshToken := req.RefreshToken
	common.ThrowIf(refreshToken == "", common.ParamsError)

	// 查 Redis 验证 Refresh Token
	key := constants.RefreshTokenKeyPrefix + refreshToken
	userIdStr, err := l.svcCtx.Redis.Get(key)
	common.ThrowIf(err != nil || userIdStr == "", common.NotLoginError)

	userId, _ := strconv.ParseInt(userIdStr, 10, 64)

	// 生成新 Access Token
	newAccessToken := utils.GenerateAccessToken(userId)

	l.svcCtx.Redis.Setex(constants.AccessTokenKeyPrefix+newAccessToken, userIdStr, constants.AccessTokenExpireMinutes*60)

	return newAccessToken, nil
}
