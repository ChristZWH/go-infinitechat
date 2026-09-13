package message

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MessageModel = (*customMessageModel)(nil)

type (
	// MessageModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMessageModel.
	MessageModel interface {
		messageModel
		FindBySessionBeforeTime(ctx context.Context, sessionId int64, beforeTIme time.Time, limit int) ([]*Message, error)
	}

	customMessageModel struct {
		*defaultMessageModel
	}
)

// NewMessageModel returns a model for the database table.
func NewMessageModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MessageModel {
	return &customMessageModel{
		defaultMessageModel: newMessageModel(conn, c, opts...),
	}
}

func (m *customMessageModel) FindBySessionBeforeTime(ctx context.Context, sessionId int64, beforeTIme time.Time, limit int) ([]*Message, error) {
	var resp []*Message
	query := fmt.Sprintf("SELECT message_id, sender_id, session_id, type, content, reply_id, session_type, created_time FROM %s WHERE session_id = ? AND created_time < ? ORDER BY created_time DESC LIMIT ?", m.table)

	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, sessionId, beforeTIme, limit)
	return resp, err
}
