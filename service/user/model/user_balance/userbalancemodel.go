package user_balance

import (
	"context"
	"database/sql"
	"fmt"
	"go-infinitechat/common/model/txctx"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserBalanceModel = (*customUserBalanceModel)(nil)

type (
	// UserBalanceModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserBalanceModel.
	UserBalanceModel interface {
		userBalanceModel
		InsertTx(ctx context.Context, data *UserBalance) (sql.Result, error)
		UpdateTx(ctx context.Context, data *UserBalance) error
		DeleteTx(ctx context.Context, userBalanceId int64) error
		DeductBalanceTx(ctx context.Context, userId, amount int64) (int64, error) // 返回 affected rows
		AddBalanceTx(ctx context.Context, userId, amount int64) error
	}

	customUserBalanceModel struct {
		*defaultUserBalanceModel
	}
)

// NewUserBalanceModel returns a model for the database table.
func NewUserBalanceModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) UserBalanceModel {
	return &customUserBalanceModel{
		defaultUserBalanceModel: newUserBalanceModel(conn, c, opts...),
	}
}

// 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customUserBalanceModel) InsertTx(ctx context.Context, data *UserBalance) (sql.Result, error) {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("insert into %s (%s) values (?,?,?,?)", m.table, userBalanceRowsExpectAutoSet)
		return session.ExecCtx(ctx, query, data.UserId, data.Balance, data.CreatedTime, data.UpdatedTime)
	}
	return m.Insert(ctx, data)
}

// 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customUserBalanceModel) UpdateTx(ctx context.Context, data *UserBalance) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("update %s set %s where `user_id` = ?", m.table, userBalanceRowsWithPlaceHolder)
		_, err := session.ExecCtx(ctx, query, data.Balance, data.CreatedTime, data.UpdatedTime, data.UserId)
		return err
	}
	return m.Update(ctx, data)
}

// 如果 ctx 中有事务 session 则走事务连接，否则走原来的 CachedConn
func (m *customUserBalanceModel) DeleteTx(ctx context.Context, userBalanceId int64) error {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("delete from %s where `user_id` = ?", m.table)
		_, err := session.ExecCtx(ctx, query, userBalanceId)
		return err
	}
	return m.Delete(ctx, userBalanceId)
}

// 扣减余额，返回受影响行数（0 表示余额不足）
func (m *customUserBalanceModel) DeductBalanceTx(ctx context.Context, userId, amount int64) (int64, error) {
	query := fmt.Sprintf("UPDATE %s SET `balance` = `balance` - ?, `updateed_time` = NOW() WHERE `user_id` = ? AND `balance` >= ?", m.table)
	if session := txctx.GetSession(ctx); session != nil {
		result, err := session.ExecCtx(ctx, query, amount, userId, amount)
		if err != nil {
			return 0, err
		}
		return result.RowsAffected()
	}
	// 非事务时需要清理缓存
	key := fmt.Sprintf("%s%v", cacheUserBalanceUserIdPrefix, userId)
	result, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, amount, userId, amount)
	}, key)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// 增加余额
func (m *customUserBalanceModel) AddBalanceTx(ctx context.Context, userId, amount int64) error {
	query := fmt.Sprintf("UPDATE %s SET `balance` = `balance` + ? WHERE `user_id` = ? ", m.table)
	if session := txctx.GetSession(ctx); session != nil {
		_, err := session.ExecCtx(ctx, query, amount, userId)
		return err
	}
	// 非事务
	key := fmt.Sprintf("%s%v", cacheUserBalanceUserIdPrefix, userId)
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, amount, userId)
	}, key)
	return err
}
