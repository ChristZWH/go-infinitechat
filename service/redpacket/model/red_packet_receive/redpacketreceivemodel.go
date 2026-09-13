package red_packet_receive

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ RedPacketReceiveModel = (*customRedPacketReceiveModel)(nil)

type (
	// RedPacketReceiveModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRedPacketReceiveModel.
	RedPacketReceiveModel interface {
		redPacketReceiveModel
		// 按红包ID查询所有领取记录
		FindByRedPacketId(ctx context.Context, redPacketId int64) ([]*RedPacketReceive, error)
		// 按红包ID分页查询领取记录
		FindPageByRedPacketId(ctx context.Context, redPacketId int64, pageNum, pageSize int) ([]*RedPacketReceive, error)
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

func (m *customRedPacketReceiveModel) FindByRedPacketId(ctx context.Context, redPacketId int64) ([]*RedPacketReceive, error) {
	query := fmt.Sprintf("select %s from %s where `red_packet_id` = ? order by `received_at` desc", redPacketReceiveRows, m.table)
	var resp []*RedPacketReceive
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, redPacketId)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *customRedPacketReceiveModel) FindPageByRedPacketId(ctx context.Context, redPacketId int64, pageNum, pageSize int) ([]*RedPacketReceive, error) {
	offset := (pageNum - 1) * pageSize
	query := fmt.Sprintf("select %s from %s where `red_packet_id` = ? order by `received_at` desc, `receiver_id` asc limit ?, ?", redPacketReceiveRows, m.table)
	var resp []*RedPacketReceive
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, redPacketId, offset, pageSize)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
