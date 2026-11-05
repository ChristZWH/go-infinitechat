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
	// 用 jwt.NewNumericDate 而不是裸 time.Time：
	// 前者会按库的 TimePrecision 截断到秒，后者会序列化出带纳秒的小数，
	// 校验方按整秒比较时会判成 "Token used before issued"。
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(userId, 10),
		"iat": jwt.NewNumericDate(now),
		"exp": jwt.NewNumericDate(now.Add(constants.AccessTokenExpireMinutes * time.Minute)),
	}

	res, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(constants.TokenSecretKey))
	common.ThrowIfWithMsg(err != nil, common.SystemError, "JWT 生成失败", err)
	return res
}
