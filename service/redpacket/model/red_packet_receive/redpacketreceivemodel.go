package red_packet_receive

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ RedPacketReceiveModel = (*customRedPacketReceiveModel)(nil)

type (
	// RedPacketReceiveModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRedPacketReceiveModel.
	RedPacketReceiveModel interface {
		redPacketReceiveModel
	}

	customRedPacketReceiveModel struct {
		*defaultRedPacketReceiveModel
	}
)

// NewRedPacketReceiveModel returns a model for the database table.
func NewRedPacketReceiveModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) RedPacketReceiveModel {
	return &customRedPacketReceiveModel{
		defaultRedPacketReceiveModel: newRedPacketReceiveModel(conn, c, opts...),
	}
}
