package balance_log

import (
	"context"
	"database/sql"
	"fmt"
	"go-infinitechat/common/model/txctx"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ BalanceLogModel = (*customBalanceLogModel)(nil)

type (
	// BalanceLogModel is an interface to be customized, add more methods here,
	// and implement the added methods in customBalanceLogModel.
	BalanceLogModel interface {
		balanceLogModel
		InsertTx(ctx context.Context, data *BalanceLog) (sql.Result, error)
		UpdateTx(ctx context.Context, data *BalanceLog) error
		DeleteTx(ctx context.Context, balanceLogId int64) error
		// 按用户ID分页查询，按创建时间倒序
		FindPageByUserIdTx(ctx context.Context, userId int64, pageNum, pageSize int) ([]*BalanceLog, error)
		// 统计用户的记录总数
		CountByUserIdTx(ctx context.Context, userId int64) (int64, error)
	}

	customBalanceLogModel struct {
		*defaultBalanceLogModel
	}
)

// NewBalanceLogModel returns a model for the database table.
func NewBalanceLogModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) BalanceLogModel {
	return &customBalanceLogModel{
		defaultBalanceLogModel: newBalanceLogModel(conn, c, opts...),
	}
}

// InsertTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customBalanceLogModel) InsertTx(ctx context.Context, data *BalanceLog) (sql.Result, error) {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?, ?, ?, ?)", m.table, balanceLogRowsExpectAutoSet)
		return session.ExecCtx(ctx, query, data.BalanceLogId, data.UserId, data.Amount, data.Type, data.RelatedId, data.CreatedTime, data.UpdatedTime)
	}
	return m.Insert(ctx, data)
}

// UpdateTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customBalanceLogModel) UpdateTx(ctx context.Context, data *BalanceLog) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("update %s set %s where `balance_log_id` = ?", m.table, balanceLogRowsWithPlaceHolder)
		_, err := session.ExecCtx(ctx, query, data.UserId, data.Amount, data.Type, data.RelatedId, data.CreatedTime, data.UpdatedTime, data.BalanceLogId)
		return err
	}
	return m.Update(ctx, data)
}

// DeleteTx 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customBalanceLogModel) DeleteTx(ctx context.Context, balanceLogId int64) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("delete from %s where `balance_log_id` = ?", m.table)
		_, err := session.ExecCtx(ctx, query, balanceLogId)
		return err
	}
	return m.Delete(ctx, balanceLogId)
}

func (m *(customBalanceLogModel)) FindPageByUserIdTx(ctx context.Context, userId int64, pageNum, pageSize int) ([]*BalanceLog, error) {
	offset := (pageNum - 1) * pageSize
	query := fmt.Sprintf("select %s from %s where `user_id` = ? order by `created_time` desc limit ?, ?", balanceLogRows, m.table)

	var resp []*BalanceLog
	err := m.CachedConn.QueryRowsNoCacheCtx(ctx, &resp, query, userId, offset, pageSize)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// CountByUserId 统计总数，用于分页计算
func (m *customBalanceLogModel) CountByUserIdTx(ctx context.Context, userId int64) (int64, error) {
	query := fmt.Sprintf("select count(*) from %s where `user_id` = ?", m.table)
	var count int64
	err := m.CachedConn.QueryRowNoCacheCtx(ctx, &count, query, userId)
	if err != nil {
		return 0, err
	}
	return count, nil
}
