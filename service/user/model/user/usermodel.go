package user

import (
	"context"
	"database/sql"
	"fmt"
	"go-infinitechat/common/model/txctx"
	"strconv"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserModel = (*customUserModel)(nil)

type (
	// UserModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserModel.
	UserModel interface {
		userModel
		InsertTx(ctx context.Context, user *User) (sql.Result, error)
		UpdateTx(ctx context.Context, user *User) error
		DeleteTx(ctx context.Context, userId int64) error
		UpdateAvatarByUserId(ctx context.Context, id int64, uri string) error
		UpdatePasswordByUserId(ctx context.Context, data *User) error
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

func (m *customUserModel) UpdatePasswordByUserId(ctx context.Context, data *User) error {
	//如果想自定义sql操作，要注意go-zero的缓存问题。比如这个更新密码的操作，如果不清除缓存，那么登录等情况的时候会出现新密码无法登录，旧密码却可以登录
	userEmailKey := cacheUserEmailPrefix + data.Email.String
	userPhoneKey := cacheUserPhonePrefix + data.Phone.String
	userUserIdKey := cacheUserUserIdPrefix + strconv.FormatInt(data.UserId, 10)
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		query := "UPDATE `user` SET `password` = ? WHERE `user_id` = ?"
		return conn.ExecCtx(ctx, query, data.Password, data.UserId)
	}, userEmailKey, userPhoneKey, userUserIdKey)
	return err
}

// InsertTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customUserModel) InsertTx(ctx context.Context, data *User) (sql.Result, error) {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", m.table, userRowsExpectAutoSet)
		return session.ExecCtx(ctx, query, data.UserId, data.Phone, data.Email, data.Password, data.Nickname, data.Avatar, data.Gender, data.Description, data.State, data.Role, data.CreatedTime, data.UpdatedTime, data.IsDelete)
	}
	return m.Insert(ctx, data)
}

// UpdateTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customUserModel) UpdateTx(ctx context.Context, data *User) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("update %s set %s where `user_id` = ?", m.table, userRowsWithPlaceHolder)
		_, err := session.ExecCtx(ctx, query, data.Phone, data.Email, data.Password, data.Nickname, data.Avatar, data.Gender, data.Description, data.State, data.Role, data.CreatedTime, data.UpdatedTime, data.IsDelete, data.UserId)
		return err
	}
	return m.Update(ctx, data)
}

// DeleteTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customUserModel) DeleteTx(ctx context.Context, userId int64) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("delete from %s where `user_id` = ?", m.table)
		_, err := session.ExecCtx(ctx, query, userId)
		return err
	}
	return m.Delete(ctx, userId)
}
