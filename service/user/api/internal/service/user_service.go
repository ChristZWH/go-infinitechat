package service

import (
	"context"
	"crypto/md5"
	"database/sql"
	"fmt"
	"go-infinitechat/common/common"
	utils2 "go-infinitechat/common/utils"
	"go-infinitechat/service/user/api/internal/types/constants"
	"go-infinitechat/service/user/api/internal/types/dto"
	"go-infinitechat/service/user/api/internal/utils"
	"go-infinitechat/service/user/model/user"
	"strconv"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type UserService struct {
	userModel user.UserModel
	rds       *redis.Redis // userService 中需要用到缓存
	// userService 中还需要什么变量继续往里面添加
}

func NewUserService(user user.UserModel, r *redis.Redis) *UserService {
	return &UserService{
		userModel: user,
		rds:       r,
	}
}

func (s *UserService) GetUser(account string) (*user.User, error) {
	if constants.PhoneRegex.MatchString(account) {
		user, err := s.userModel.FindOneByPhone(context.Background(), sql.NullString{
			String: account,
			Valid:  true,
		})
		if err != nil {
			return nil, err
		}
		return user, nil
	} else if constants.EmailRegex.MatchString(account) {
		user, err := s.userModel.FindOneByEmail(context.Background(), sql.NullString{
			String: account,
			Valid:  true,
		})
		if err != nil {
			return nil, err
		}
		return user, nil
	}
	return nil, nil
}

func (s *UserService) GetTwoToken(userId int64) dto.TwoJWT {
	// 踢掉旧的会话（单点登录）
	userIdStr := strconv.FormatInt(userId, 10)

	indexKey := constants.UserRefreshTokenKeyPrefix + userIdStr
	oldRefreshToken, _ := s.rds.Get(indexKey)
	if oldRefreshToken != "" {
		s.rds.Del(oldRefreshToken)
		s.rds.Del(indexKey)
		common.Infof("用户 %s 被踢下线，旧 Refresh Token: %s\n", userIdStr, oldRefreshToken)
	}

	// 生成Token
	accessToken := utils.GenerateAccessToken(userId)
	refreshToken := utils2.NextString()

	s.rds.Setex(constants.AccessTokenKeyPrefix+accessToken, userIdStr, constants.AccessTokenExpireMinutes*60)
	s.rds.Setex(constants.RefreshTokenKeyPrefix+refreshToken, userIdStr, constants.RefreshTokenExpireMinutes*60)
	s.rds.Setex(constants.UserRefreshTokenKeyPrefix+userIdStr, constants.RefreshTokenKeyPrefix+refreshToken, constants.RefreshTokenExpireMinutes*60)

	common.Infof("用户: %s, accessToken: %s, refreshToken: %s", userIdStr, accessToken, refreshToken)

	return dto.TwoJWT{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
}

func (s *UserService) EncryptPassword(password string) string {
	h := md5.New()
	h.Write([]byte(constants.PasswordSalt + password))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (s *UserService) GetUserById(userId int64) (*user.User, error) {
	return s.userModel.FindOne(context.Background(), userId)
}

func (s *UserService) UpdatePassword(ctx context.Context, u *user.User) {
	err := s.userModel.UpdatePasswordByUserId(ctx, u)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "密码修改失败", err)
}

func (s *UserService) UpdateAvatar(ctx context.Context, id int64, uri string) {
	err := s.userModel.UpdateAvatarByUserId(ctx, id, uri)
	common.ThrowIfWithMsg(err != nil, common.OperationError, "更新头像失败")
}
