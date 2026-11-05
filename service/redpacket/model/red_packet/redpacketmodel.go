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
		// MarkExpiredIfNotCompleted 条件更新：仅当 status = notCompletedStatus 时才置为 expiredStatus。
		// 返回受影响行数：1 表示本次抢到了「未领完 → 已过期」的状态迁移，可以继续退款；
		// 0 表示已被其它消费者处理过，必须跳过退款（防止 Kafka 重投/并发消费重复退款）。
		MarkExpiredIfNotCompleted(ctx context.Context, redPacketId, expiredStatus, notCompletedStatus int64) (int64, error)
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

// MarkExpiredIfNotCompleted 见接口注释。
// 关键点：状态判断和状态写入是同一条 SQL，不存在「先查后写」的窗口。
func (m *customRedPacketModel) MarkExpiredIfNotCompleted(ctx context.Context, redPacketId, expiredStatus, notCompletedStatus int64) (int64, error) {
	query := fmt.Sprintf("update %s set `status` = ?, `updated_time` = now() where `red_packet_id` = ? and `status` = ?", m.table)

	// 事务内：走 session，缓存由调用方在提交后清理
	if session := txctx.GetSession(ctx); session != nil {
		res, err := session.ExecCtx(ctx, query, expiredStatus, redPacketId, notCompletedStatus)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}

	// 非事务：走 ExecCtx，自动失效 red_packet_id 缓存
	key := fmt.Sprintf("%s%v", cacheRedPacketRedPacketIdPrefix, redPacketId)
	res, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, expiredStatus, redPacketId, notCompletedStatus)
	}, key)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
