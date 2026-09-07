// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"
	"errors"
	"strconv"

	"go-infinitechat/common/common"
	commonconstants "go-infinitechat/common/model/constants"
	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/types/constants"

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
	if req.Type == "password" {
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
		l.svcCtx.Redis.Hset(commonconstants.RedisWsServerUri, strconv.FormatInt(user.UserId, 10), wsServerUri)

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
	} else if req.Type == "code" {
		loginCodeKey := ""
		if constants.PhoneRegex.MatchString(req.Account) {
			code := "123456" // 写死123456
			if code != req.Code {
				return nil, common.LoginError
			}
		} else if constants.EmailRegex.MatchString(req.Account) {
			loginCodeKey = constants.LoginCodeEmailPrefix + req.Account
			code, _ := l.svcCtx.Redis.Get(loginCodeKey)
			if code == "" || code != req.Code {
				return nil, common.LoginError
			}
		} else {
			return nil, common.PhoneEmailError
		}
		// 验证码校验通过
		// 验证用户是否存在
		user, err := l.svcCtx.UserService.GetUser(req.Account)
		if err != nil {
			if errors.Is(err, sqlx.ErrNotFound) {
				return nil, common.WrapError(common.UserNotExists, err)
			}
			return nil, common.WrapError(common.SystemError, err)
		}

		// 删除 redis 保存的验证码
		if loginCodeKey != "" {
			l.svcCtx.Redis.Del(loginCodeKey)
		}

		twoToken := l.svcCtx.UserService.GetTwoToken(user.UserId)
		wsServerUri := l.svcCtx.WsServerLocator.GetWsServerUri(strconv.FormatInt(user.UserId, 10))
		l.svcCtx.Redis.Hset(commonconstants.RedisWsServerUri, strconv.FormatInt(user.UserId, 10), wsServerUri)

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
	return nil, common.ParamsError
}
