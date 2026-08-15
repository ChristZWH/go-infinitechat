package user_session

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserSessionModel = (*customUserSessionModel)(nil)

type (
	// UserSessionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserSessionModel.
	UserSessionModel interface {
		userSessionModel
	}

	customUserSessionModel struct {
		*defaultUserSessionModel
	}
)

// NewUserSessionModel returns a model for the database table.
func NewUserSessionModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) UserSessionModel {
	return &customUserSessionModel{
		defaultUserSessionModel: newUserSessionModel(conn, c, opts...),
	}
}
