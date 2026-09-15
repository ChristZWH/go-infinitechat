package red_packet

import (
	"context"
	"database/sql"
	"fmt"

	"go-infinitechat/common/model/txctx"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ RedPacketModel = (*customRedPacketModel)(nil)

type (
	// RedPacketModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRedPacketModel.
	RedPacketModel interface {
		redPacketModel
		InsertTx(ctx context.Context, data *RedPacket) (sql.Result, error)
		UpdateTx(ctx context.Context, data *RedPacket) error
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

func (m *customRedPacketModel) InsertTx(ctx context.Context, data *RedPacket) (sql.Result, error) {
	if session := txctx.GetSession(ctx); session != nil {
		query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", m.table, redPacketRowsExpectAutoSet)
		return session.ExecCtx(ctx, query, data.RedPacketId, data.SenderId, data.SessionId, data.SessionType, data.RedPacketWrapperText, data.RedPacketType, data.TotalAmount, data.TotalCount, data.Status, data.CreatedTime, data.UpdatedTime)
	}
	return m.Insert(ctx, data)
}

func (m *customRedPacketModel) UpdateTx(ctx context.Context, data *RedPacket) error {
	query := fmt.Sprintf("update %s set %s where `red_packet_id` = ?", m.table, redPacketRowsWithPlaceHolder)
	if session := txctx.GetSession(ctx); session != nil {
		// 占位符顺序 = redPacketRowsWithPlaceHolder 的列顺序（除主键外所有字段，最后补主键）
		_, err := session.ExecCtx(ctx, query,
			data.SenderId, data.SessionId, data.SessionType,
			data.RedPacketWrapperText, data.RedPacketType,
			data.TotalAmount, data.TotalCount, data.Status,
			data.CreatedTime, data.UpdatedTime, data.RedPacketId)
		return err
	}
	// 非事务走生成的 Update，自带缓存失效
	return m.Update(ctx, data)
}
