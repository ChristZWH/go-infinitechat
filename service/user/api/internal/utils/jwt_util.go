package utils

import (
	"encoding/base64"
	"go-infinitechat/service/user/api/internal/types/constants"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// 获取签名密钥（从常量中读取，避免硬编码）
func getSignInKey() []byte {
	keyBytes, err := base64.StdEncoding.DecodeString(constants.TokenSecretKey)
	if err != nil {
		return nil
	}
	return keyBytes
}

// 生成JWT 支持自定义过期时间
func GenerateAccessToken(userId int64) string {
	now := time.Now()
	claims := jwt.MapClaims{
		"subject":    strconv.FormatInt(userId, 10),
		"issueAt":    now,
		"expiration": now.Add(constants.AccessTokenExpireMinutes * time.Minute),
	}

	res, err := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(constants.TokenSecretKey)
	if err != nil {
		return ""
	}
	return res
}
