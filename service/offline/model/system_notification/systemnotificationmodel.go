package system_notification

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SystemNotificationModel = (*customSystemNotificationModel)(nil)

type (
	// SystemNotificationModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSystemNotificationModel.
	SystemNotificationModel interface {
		systemNotificationModel
		FindUnreadByReceiverId(ctx context.Context, receiverId int64, offset, limit int) ([]*SystemNotification, error)
		CountUnreadByReceiverId(ctx context.Context, receiverId int64) (int64, error)
		MarkAsRead(ctx context.Context, userId, notificationId int64) (int64, error)
		MarkAsReadBatch(ctx context.Context, userId int64, ids []int64) (int64, error)
		MarkAllAsRead(ctx context.Context, userId int64) (int64, error)
	}

	customSystemNotificationModel struct {
		*defaultSystemNotificationModel
	}
)

// NewSystemNotificationModel returns a model for the database table.
func NewSystemNotificationModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SystemNotificationModel {
	return &customSystemNotificationModel{
		defaultSystemNotificationModel: newSystemNotificationModel(conn, c, opts...),
	}
}

func (m *customSystemNotificationModel) FindUnreadByReceiverId(ctx context.Context, receiverId int64, offset, limit int) ([]*SystemNotification, error) {
	var resp []*SystemNotification
	query := fmt.Sprintf("SELECT * FROM %s WHERE receiver_id = ? AND is_read = 0 ORDER BY created_time DESC LIMIT ?, ?", m.table)
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, receiverId, offset, limit)
	return resp, err
}

func (m *customSystemNotificationModel) CountUnreadByReceiverId(ctx context.Context, receiverId int64) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE receiver_id = ? AND is_read = 0", m.table)
	err := m.QueryRowNoCacheCtx(ctx, &count, query, receiverId)
	return count, err
}

func (m *customSystemNotificationModel) MarkAsRead(ctx context.Context, userId, notificationId int64) (int64, error) {
	query := fmt.Sprintf("UPDATE %s SET is_read = 1, updated_time = NOW() WHERE id = ? AND receiver_id = ? AND is_read = 0", m.table)
	resp, err := m.ExecNoCacheCtx(ctx, query, notificationId, userId)
	if err != nil {
		return 0, err
	}
	return resp.RowsAffected()
}

func (m *customSystemNotificationModel) MarkAsReadBatch(ctx context.Context, userId int64, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	// 构建 IN 子句占位符
	placeholders := ""
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, userId)
	for i, id := range ids {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, id)
	}
	query := fmt.Sprintf("UPDATE %s SET is_read = 1, updated_time = NOW() WHERE receiver_id = ? AND is_read = 0 AND id IN (%s)", m.table, placeholders)
	resp, err := m.ExecNoCacheCtx(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return resp.RowsAffected()
}

func (m *customSystemNotificationModel) MarkAllAsRead(ctx context.Context, userId int64) (int64, error) {
	query := fmt.Sprintf("UPDATE %s SET is_read = 1, updated_time = NOW() WHERE receiver_id = ? AND is_read = 0", m.table)
	res, err := m.ExecNoCacheCtx(ctx, query, userId)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
