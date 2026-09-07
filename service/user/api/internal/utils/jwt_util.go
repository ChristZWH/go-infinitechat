package utils

import (
	"go-infinitechat/common/common"
	"go-infinitechat/service/user/api/internal/types/constants"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// 生成JWT 支持自定义过期时间
func GenerateAccessToken(userId int64) string {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(userId, 10),
		"iat": now,
		"exp": now.Add(constants.AccessTokenExpireMinutes * time.Minute),
	}

	res, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(constants.TokenSecretKey))
	common.ThrowIfWithMsg(err != nil, common.SystemError, "JWT 生成失败", err)
	return res
}
