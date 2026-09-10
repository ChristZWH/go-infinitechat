package friend

import (
	"context"
	"database/sql"
	"fmt"
	"go-infinitechat/common/model/txctx"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ FriendModel = (*customFriendModel)(nil)

type (
	// FriendModel is an interface to be customized, add more methods here,
	// and implement the added methods in customFriendModel.
	FriendModel interface {
		friendModel
		InsertTx(ctx context.Context, data *Friend) (sql.Result, error)
		UpdateTx(ctx context.Context, data *Friend) error
		DeleteTx(ctx context.Context, id int64) error
		FindListByUserIdTx(ctx context.Context, userId int64) ([]*Friend, error)
		DeleteByBothDirectionsTx(ctx context.Context, userId, friendId int64) error
		DeleteByBothDirections(ctx context.Context, userId, friendId int64) error
		UpdateStatusByUserIdFriendId(ctx context.Context, userId, friendId, status int64) error
	}

	customFriendModel struct {
		*defaultFriendModel
	}
)

// NewFriendModel returns a model for the database table.
func NewFriendModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) FriendModel {
	return &customFriendModel{
		defaultFriendModel: newFriendModel(conn, c, opts...),
	}
}

// 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customFriendModel) InsertTx(ctx context.Context, data *Friend) (sql.Result, error) {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?, ?)", m.table, friendRowsExpectAutoSet)
		return session.ExecCtx(ctx, query, data.UserId, data.FriendId, data.Status, data.CreatedTime, data.UpdatedTime)
	}
	return m.Insert(ctx, data)
}

// 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customFriendModel) UpdateTx(ctx context.Context, data *Friend) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("update %s set %s where `id` = ?", m.table, friendRowsWithPlaceHolder)
		_, err := session.ExecCtx(ctx, query, data.UserId, data.FriendId, data.Status, data.CreatedTime, data.UpdatedTime, data.Id)
		return err
	}
	return m.Update(ctx, data)
}

// 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customFriendModel) DeleteTx(ctx context.Context, id int64) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("delete from %s where `id` = ?", m.table)
		_, err := session.ExecCtx(ctx, query, id)
		return err
	}
	return m.Delete(ctx, id)
}

func (m *customFriendModel) FindListByUserIdTx(ctx context.Context, userId int64) ([]*Friend, error) {
	var resp []*Friend
	query := fmt.Sprintf("select %s from %s where `user_id` = ? and `status` != 2 order by `created_time` desc", friendRows, m.table)

	if session := txctx.GetSession(ctx); session != nil {
		err := session.QueryRowsCtx(ctx, &resp, query, userId)
		return resp, err
	}

	err := m.CachedConn.QueryRowsNoCacheCtx(ctx, &resp, query, userId)
	return resp, err
}

func (m customFriendModel) DeleteByBothDirectionsTx(ctx context.Context, userId, friendId int64) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("delete from %s where (`user_id` = ? and `friend_id` = ?) or (`user_id` = ? and `friend_id` = ?)", m.table)
		_, err := session.ExecCtx(ctx, query, userId, friendId, friendId, userId)
		return err
	}
	return m.DeleteByBothDirections(ctx, userId, friendId)
}

// 双向删除好友关系（非事务）
func (m *customFriendModel) DeleteByBothDirections(ctx context.Context, userId, friendId int64) error {
	key1 := fmt.Sprintf("%s%v:%v", cacheFriendUserIdFriendIdPrefix, userId, friendId)
	key2 := fmt.Sprintf("%s%v:%v", cacheFriendUserIdFriendIdPrefix, friendId, userId)
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		query := fmt.Sprintf("delete from %s where (user_id = ? and friend_id = ?) or (user_id = ? and friend_id = ?)", m.table)
		return conn.ExecCtx(ctx, query, userId, friendId, friendId, userId)
	}, key1, key2)
	return err
}

// 按 userId+friendId 更新好友状态
func (m customFriendModel) UpdateStatusByUserIdFriendId(ctx context.Context, userId, friendId, status int64) error {
	friendUserIdFriendIdKey := fmt.Sprintf("%s%v:%v", cacheFriendUserIdFriendIdPrefix, userId, friendId)
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		query := fmt.Sprintf("update %s set `status` = ?, `update_time` = now() where `user_id` = ? and `friend_id` = ?", m.table)
		return conn.ExecCtx(ctx, query, status, userId, friendId)
	}, friendUserIdFriendIdKey)
	return err
}
