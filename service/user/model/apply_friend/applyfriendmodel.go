package apply_friend

import (
	"context"
	"database/sql"
	"fmt"
	"go-infinitechat/common/model/txctx"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ApplyFriendModel = (*customApplyFriendModel)(nil)

type (
	// ApplyFriendModel is an interface to be customized, add more methods here,
	// and implement the added methods in customApplyFriendModel.
	ApplyFriendModel interface {
		applyFriendModel
		InsertTx(ctx context.Context, data *ApplyFriend) (sql.Result, error)
		UpdateTx(ctx context.Context, data *ApplyFriend) error
		DeleteTx(ctx context.Context, applyFriendId int64) error
	}

	customApplyFriendModel struct {
		*defaultApplyFriendModel
	}
)

// NewApplyFriendModel returns a model for the database table.
func NewApplyFriendModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ApplyFriendModel {
	return &customApplyFriendModel{
		defaultApplyFriendModel: newApplyFriendModel(conn, c, opts...),
	}
}

func (m *customApplyFriendModel) InsertTx(ctx context.Context, data *ApplyFriend) (sql.Result, error) {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?, ?, ?, ?)", m.table, applyFriendRowsExpectAutoSet)
		return session.ExecCtx(ctx, query, data.ApplyFriendId, data.SenderId, data.ReceiverId, data.Message, data.Status, data.CreatedTime, data.UpdatedTime)
	}
	return m.Insert(ctx, data)
}

func (m *customApplyFriendModel) UpdateTx(ctx context.Context, data *ApplyFriend) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("update %s set %s where `apply_friend_id` = ?", m.table, applyFriendRowsWithPlaceHolder)
		_, err := session.ExecCtx(ctx, query, data.SenderId, data.ReceiverId, data.Message, data.Status, data.CreatedTime, data.UpdatedTime, data.ApplyFriendId)
		return err
	}
	return m.Update(ctx, data)
}

func (m *customApplyFriendModel) DeleteTx(ctx context.Context, applyFriendId int64) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("delete from %s where `apply_friend_id` = ?", m.table)
		_, err := session.ExecCtx(ctx, query, applyFriendId)
		return err
	}
	return m.Delete(ctx, applyFriendId)
}
