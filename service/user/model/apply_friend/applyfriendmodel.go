package apply_friend

import (
	"context"
	"database/sql"
	"fmt"
	"go-infinitechat/common/model/txctx"
	"strings"

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
		FindPageByUserIdWithPagination(ctx context.Context, userId int64, pageNum, pageSize int32) ([]*ApplyFriend, int64, error)
		CountByReceiverAndStatus(ctx context.Context, receiverId, status int64) (int64, error)
		UpdateStatusBySenderIdsAndReceiverId(ctx context.Context, receiverId int64, senderIds []int64, fromStatus, toStatus int64) (int64, error)
		DeleteByBothDirectionsTx(ctx context.Context, userId, friendId int64) error
		DeleteByBothDirections(ctx context.Context, userId, friendId int64) error
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

// InsertTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customApplyFriendModel) InsertTx(ctx context.Context, data *ApplyFriend) (sql.Result, error) {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?, ?, ?, ?)", m.table, applyFriendRowsExpectAutoSet)
		return session.ExecCtx(ctx, query, data.ApplyFriendId, data.SenderId, data.ReceiverId, data.Message, data.Status, data.CreatedTime, data.UpdatedTime)
	}
	return m.Insert(ctx, data)
}

// UpdateTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customApplyFriendModel) UpdateTx(ctx context.Context, data *ApplyFriend) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("update %s set %s where `apply_friend_id` = ?", m.table, applyFriendRowsWithPlaceHolder)
		_, err := session.ExecCtx(ctx, query, data.SenderId, data.ReceiverId, data.Message, data.Status, data.CreatedTime, data.UpdatedTime, data.ApplyFriendId)
		return err
	}
	return m.Update(ctx, data)
}

// DeleteTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customApplyFriendModel) DeleteTx(ctx context.Context, applyFriendId int64) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("delete from %s where `apply_friend_id` = ?", m.table)
		_, err := session.ExecCtx(ctx, query, applyFriendId)
		return err
	}
	return m.Delete(ctx, applyFriendId)
}

// 分页查询与某用户相关的好友申请（发送或接收）
func (m *customApplyFriendModel) FindPageByUserIdWithPagination(ctx context.Context, userId int64, pageNum, pageSize int32) ([]*ApplyFriend, int64, error) {
	// 先查总数
	var total int64
	query := fmt.Sprintf("select count(*) from %s where `sender_id` = ? or `receiver_id = ?`", m.table)
	err := m.QueryRowNoCacheCtx(ctx, &total, query, userId, userId)

	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	var resp []*ApplyFriend
	offset := (pageNum - 1) * pageSize
	query = fmt.Sprintf("select %s from %s where `sender_id` = ? or `receiver_id` = ? order by `updated_time` desc limit ? offset ?", applyFriendRows, m.table)
	err = m.QueryRowsNoCacheCtx(ctx, &resp, query, userId, userId, pageSize, offset)

	if err != nil {
		return nil, 0, nil
	}
	return resp, total, nil
}

// CountByReceiverAndStatus 统计指定接收者和状态的申请数量
func (m *customApplyFriendModel) CountByReceiverAndStatus(ctx context.Context, receiverId, status int64) (int64, error) {
	var count int64
	query := fmt.Sprintf("select count(*) from %s where `receiver_id` = ? and `status` = ?", m.table)
	err := m.QueryRowNoCacheCtx(ctx, &count, query, receiverId, status)
	return count, err
}

// UpdateStatusBySenderIdsAndReceiverId 没有看懂
// 把"同一接收者名下、来自一批申请者"的好友申请，从某个状态批量改成另一个状态——一条 SQL 搞定，不用循环 N 次

// UpdateStatusBySenderIdsAndReceiverId 批量更新好友申请状态（仅更新指定来源状态的记录）
func (m *customApplyFriendModel) UpdateStatusBySenderIdsAndReceiverId(ctx context.Context, receiverId int64, senderIds []int64, fromStatus, toStatus int64) (int64, error) {
	if len(senderIds) == 0 {
		return 0, nil
	}

	// 构建 IN 子句占位符
	placeholders := make([]string, len(senderIds))
	args := make([]interface{}, 0, len(senderIds)+3)
	args = append(args, toStatus, receiverId)
	for i, id := range senderIds {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, fromStatus)

	query := fmt.Sprintf("update %s set `status` = ?, `updated_time` = now() where `receiver_id` = ? and `sender_id` in (%s) and `status` = ?", m.table, strings.Join(placeholders, ","))

	result, err := m.ExecNoCacheCtx(ctx, query, args...)

	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteByBothDirections 双向删除好友申请记录（事务版）
func (m *customApplyFriendModel) DeleteByBothDirectionsTx(ctx context.Context, userId, friendId int64) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("delete from %s where (`sender_id` = ? and `receiver_id` = ?) or (`sender_id` = ? and `receiver_id` = ?)", m.table)
		_, err := session.ExecCtx(ctx, query, userId, friendId, friendId, userId)
		return err
	}
	return m.DeleteByBothDirections(ctx, userId, friendId)
}

// DeleteByBothDirections 双向删除好友申请记录（非事务）
func (m *customApplyFriendModel) DeleteByBothDirections(ctx context.Context, userId, friendId int64) error {
	query := fmt.Sprintf("delete from %s where (`sender_id` = ? and `receiver_id` = ?) or (`sender_id` = ? and `receiver_id` = ?)", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, userId, friendId, friendId, userId)
	return err
}
