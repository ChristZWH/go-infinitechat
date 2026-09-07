package session

import (
	"context"
	"database/sql"
	"fmt"
	"go-infinitechat/common/model/txctx"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SessionModel = (*customSessionModel)(nil)

type (
	// SessionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSessionModel.
	SessionModel interface {
		sessionModel
		InsertTx(ctx context.Context, data *Session) (sql.Result, error)
		UpdateTx(ctx context.Context, data *Session) error
		DeleteTx(ctx context.Context, sessionId int64) error
	}

	customSessionModel struct {
		*defaultSessionModel
	}
)

// NewSessionModel returns a model for the database table.
func NewSessionModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SessionModel {
	return &customSessionModel{
		defaultSessionModel: newSessionModel(conn, c, opts...),
	}
}

// InsertTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customSessionModel) InsertTx(ctx context.Context, data *Session) (sql.Result, error) {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?, ?, ?, ?)", m.table, sessionRowsExpectAutoSet)
		return session.ExecCtx(ctx, query, data.SessionId, data.Name, data.Type, data.Status, data.CreatedTime, data.UpdatedTime, data.Avatar)
	}
	return m.Insert(ctx, data)
}

// UpdateTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customSessionModel) UpdateTx(ctx context.Context, data *Session) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("update %s set %s where `session_id` = ?", m.table, sessionRowsWithPlaceHolder)
		_, err := session.ExecCtx(ctx, query, data.Name, data.Type, data.Status, data.CreatedTime, data.UpdatedTime, data.Avatar, data.SessionId)
		return err
	}
	return m.Update(ctx, data)
}

// DeleteTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customSessionModel) DeleteTx(ctx context.Context, sessionId int64) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("delete from %s where `session_id` = ?", m.table)
		_, err := session.ExecCtx(ctx, query, sessionId)
		return err
	}
	return m.Delete(ctx, sessionId)
}
