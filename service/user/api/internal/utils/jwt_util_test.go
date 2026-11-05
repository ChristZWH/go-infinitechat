package utils

import (
	"strconv"
	"testing"
	"time"

	"go-infinitechat/service/user/api/internal/types/constants"

	"github.com/golang-jwt/jwt/v4"
)

func TestGenerateAccessTokenClaims(t *testing.T) {
	const userId int64 = 123456789

	tokenStr := GenerateAccessToken(userId)
	if tokenStr == "" {
		t.Fatal("生成的 token 为空")
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			t.Fatalf("签名算法 = %v, 期望 HMAC", token.Header["alg"])
		}
		return []byte(constants.TokenSecretKey), nil
	})
	if err != nil {
		t.Fatalf("token 校验失败: %v", err)
	}
	if !token.Valid {
		t.Fatal("token 无效")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims 类型 = %T", token.Claims)
	}

	if sub, _ := claims["sub"].(string); sub != strconv.FormatInt(userId, 10) {
		t.Errorf("sub = %v, 期望 %d", claims["sub"], userId)
	}

	iat, okIat := claims["iat"].(float64)
	exp, okExp := claims["exp"].(float64)
	if !okIat || !okExp {
		t.Fatalf("缺少 iat/exp: %+v", claims)
	}

	wantExpire := iat + float64(constants.AccessTokenExpireMinutes*60)
	if diff := exp - wantExpire; diff > 1 || diff < -1 {
		t.Errorf("过期时间与 AccessTokenExpireMinutes 不一致: exp-iat=%.0fs, 期望 %ds",
			exp-iat, constants.AccessTokenExpireMinutes*60)
	}

	if exp < float64(time.Now().Unix()) {
		t.Error("新签发的 token 已经过期")
	}
}

func TestGenerateAccessTokenUniquePerCall(t *testing.T) {
	// iat 是秒级，同一秒内两次调用 claims 可能相同，但签名结果不参与去重；
	// 这里只保证不会 panic 且都可用。
	a := GenerateAccessToken(1)
	b := GenerateAccessToken(2)
	if a == b {
		t.Error("不同 userId 的 token 不应相同")
	}
}

func TestTokenSignatureActuallyVerified(t *testing.T) {
	tokenStr := GenerateAccessToken(42)

	// 用错误密钥解析必须失败：证明签名是真实生效的，而不是随便一段字符串
	_, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte("wrong-secret"), nil
	})
	if err == nil {
		t.Fatal("用错误密钥竟然校验通过，签名未生效")
	}
}
