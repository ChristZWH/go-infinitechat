package user

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserModel = (*customUserModel)(nil)

type (
	// UserModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserModel.
	UserModel interface {
		userModel
		UpdateAvatarByUserId(ctx context.Context, id int64, uri string) error
	}

	customUserModel struct {
		*defaultUserModel
	}
)

// NewUserModel returns a model for the database table.
func NewUserModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) UserModel {
	return &customUserModel{
		defaultUserModel: newUserModel(conn, c, opts...),
	}
}

func (m *customUserModel) UpdateAvatarByUserId(ctx context.Context, id int64, uri string) error {
	userUserIdKey := fmt.Sprintf("%s%v", cacheUserUserIdPrefix, id)
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		query := "UPDATE `user` SET `avatar` = ? WHERE `user_id` = ?"
		return conn.ExecCtx(ctx, query, uri, id)
	}, userUserIdKey)
	return err
}
