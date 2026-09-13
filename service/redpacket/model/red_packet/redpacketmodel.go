package red_packet

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ RedPacketModel = (*customRedPacketModel)(nil)

type (
	// RedPacketModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRedPacketModel.
	RedPacketModel interface {
		redPacketModel
	}

	customRedPacketModel struct {
		*defaultRedPacketModel
	}
)

// NewRedPacketModel returns a model for the database table.
func NewRedPacketModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) RedPacketModel {
	return &customRedPacketModel{
		defaultRedPacketModel: newRedPacketModel(conn, c, opts...),
	}
}
