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
		// 按红包ID聚合统计领取数量与总金额（轻量查询，只返回一行）
		CountSumByRedPacketId(ctx context.Context, redPacketId int64) (count int, sumFen int64, err error)
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

// 全量 一次性拉取所有记录
func (m *customRedPacketReceiveModel) FindByRedPacketId(ctx context.Context, redPacketId int64) ([]*RedPacketReceive, error) {
	query := fmt.Sprintf("select %s from %s where `red_packet_id` = ? order by `received_at` desc", redPacketReceiveRows, m.table)
	var resp []*RedPacketReceive
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, redPacketId)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// 分页 每次只有查询一页 limit
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

func (m *customRedPacketReceiveModel) CountSumByRedPacketId(ctx context.Context, redPacketId int64) (count int, sumFen int64, err error) {
	query := fmt.Sprintf("select count(*) as `cnt`, coalesce(sum(`amount`), 0) as `total` from %s where `red_packet_id` = ?", m.table)
	var resp struct {
		Cnt   int64 `db:"cnt"`
		Total int64 `db:"total"`
	}
	if err := m.QueryRowNoCacheCtx(ctx, &resp, query, redPacketId); err != nil {
		return 0, 0, err
	}
	count, sumFen = int(resp.Cnt), resp.Total
	return
}
