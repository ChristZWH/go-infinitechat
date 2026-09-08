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

		// 查询某个会话（群聊）下的所有成员 userId 列表
		// 返回的是 []int64，只包含状态正常（status=0）的成员
		FindMemberIdsBySessionId(ctx context.Context, sessionId int64) ([]int64, error)
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

// 查询某个会话下所有正常状态的成员 userId
// 使用场景：群聊消息推送时，需要知道群里有哪些成员，才能逐个推送
// 参数:
//
//	sessionId - 会话ID（群聊ID）
//
// 返回:
//
//	[]int64 - 该群所有正常成员的 userId 列表
//	error   - 数据库查询错误
//
// 注意：使用 QueryRowsNoCacheCtx（不走缓存），因为：
//  1. 群成员列表是多行结果，不适合单行缓存模式
//  2. 群成员可能频繁变动（加入/退出/踢人），缓存一致性难保证
func (m *customUserSessionModel) FindMemberIdsBySessionId(ctx context.Context, sessionId int64) ([]int64, error) {
	query := fmt.Sprintf("select `user_id` from %s where `user_id` = ? and `status` = 0", m.table)
	var members []struct {
		UserId int64 `db:"user_id"`
	}

	// 用临时结构体接收查询结果（只需要 user_id 一个字段）
	if err := m.QueryRowsNoCacheCtx(ctx, &members, query, sessionId); err != nil {
		return nil, err
	}

	// 提取 userId 列表
	userIds := make([]int64, 0, len(members))
	for _, m := range members {
		userIds = append(userIds, m.UserId)
	}

	return userIds, nil
}
