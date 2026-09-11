package system_notification

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SystemNotificationModel = (*customSystemNotificationModel)(nil)

type (
	// SystemNotificationModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSystemNotificationModel.
	SystemNotificationModel interface {
		systemNotificationModel
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
