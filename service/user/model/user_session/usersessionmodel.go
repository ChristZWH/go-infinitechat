package user_session

import (
	"context"
	"database/sql"
	"fmt"
	"go-infinitechat/common/model/txctx"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserSessionModel = (*customUserSessionModel)(nil)

type (
	// UserSessionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserSessionModel.
	UserSessionModel interface {
		userSessionModel
		InsertTx(ctx context.Context, data *UserSession) (sql.Result, error)
		UpdateTx(ctx context.Context, data *UserSession) error
		DeleteTx(ctx context.Context, userId int64, sessionId int64) error
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

// InsertTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customUserSessionModel) InsertTx(ctx context.Context, data *UserSession) (sql.Result, error) {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("insert into %s (`user_id`,`session_id`,`role`,`status`) values (?, ?, ?, ?)", m.table)
		return session.ExecCtx(ctx, query, data.UserId, data.SessionId, data.Role, data.Status)
	}
	return m.Insert(ctx, data)
}

// UpdateTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customUserSessionModel) UpdateTx(ctx context.Context, data *UserSession) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("update %s set `role` = ?, `status` = ? where `user_id` = ? and `session_id` = ?", m.table)
		_, err := session.ExecCtx(ctx, query, data.Role, data.Status, data.UserId, data.SessionId)
		return err
	}
	return m.Update(ctx, data)
}

// DeleteTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customUserSessionModel) DeleteTx(ctx context.Context, userId int64, sessionId int64) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("delete from %s where `user_id` = ? and `session_id` = ?", m.table)
		_, err := session.ExecCtx(ctx, query, userId, sessionId)
		return err
	}
	return m.Delete(ctx, userId, sessionId)
}
