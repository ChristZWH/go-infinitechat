package constants

import "time"

const (
	AccessTokenExpireMinutes  = 30
	RefreshTokenExpireMinutes = 7 * 24 * 60     // 7 天
	RenewThreshold            = 5 * time.Minute // 这个值也还没有使用
	AccessTokenKeyPrefix      = "access_token:"
	RefreshTokenKeyPrefix     = "refresh_token:"
	UserRefreshTokenKeyPrefix = "user:refresh:"
	TokenSecretKey            = "6J8v9C2gX5qR1tY7zUw3eD0fA4bK7hN2mP5lO9iI1jG6kL8nV4cB3yT0xQ"
)
