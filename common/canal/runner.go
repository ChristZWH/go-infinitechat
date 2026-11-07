package canal

import (
	"context"
	"time"

	"go-infinitechat/common/common"

	"github.com/gogo/protobuf/proto"
	"github.com/withlin/canal-go/client"
	pbe "github.com/withlin/canal-go/protocol/entry"
)

const (
	defaultBatchSize = 1000
	// 长轮询超时：最多阻塞等 30 秒，有数据立即返回；单位毫秒（units 传 nil 用默认）
	defaultTimeoutMs = 30 * 1000
	// soTimeOut / idleTimeOut 沿用原实现
	soTimeOut   = 60000
	idleTimeOut = 0
)

// Runner Canal 消费者骨架：负责连接生命周期、拉取、Ack、按表分发。
type Runner struct {
	config   Config
	handlers map[string]Handler // "schema.table" -> handler
}

func NewRunner(cfg Config) *Runner {
	return &Runner{
		config:   cfg,
		handlers: make(map[string]Handler),
	}
}

// RegisterHandler 注册某张表的处理器，key 为 "schema.table"。
func (r *Runner) RegisterHandler(schema, table string, h Handler) {
	r.handlers[schema+"."+table] = h
}

// Start 阻塞运行，直到 ctx 取消。
func (r *Runner) Start(ctx context.Context) {
	connector := client.NewSimpleCanalConnector(
		r.config.Address,
		r.config.Port,
		r.config.Username,
		r.config.Password,
		r.config.Destination,
		soTimeOut,
		idleTimeOut,
	)

	if err := connector.Connect(); err != nil {
		common.Errorf("Canal 连接失败: %s", err.Error())
		return
	}
	defer connector.DisConnection()

	if err := connector.Subscribe(r.config.Filter); err != nil {
		common.Errorf("Canal 订阅失败: %s", err.Error())
		return
	}
	common.Infof("Canal 订阅成功，destination=%s, filter=%s", r.config.Destination, r.config.Filter)

	for {
		select {
		case <-ctx.Done():
			common.Info("Canal 客户端已停止")
			return
		default:
		}

		// 关键：必须用 GetWithOutAck，而不是 Get。
		// canal-go 的 Get 内部是「GetWithOutAck + 立刻 Ack」——Ack 发生在处理之前，
		// 处理中途崩溃这一批就永久丢失（at-most-once）。
		// 这里改成拉取 → 处理成功 → 再 Ack，实现 at-least-once。
		var timeoutMs int64 = defaultTimeoutMs
		msg, err := connector.GetWithOutAck(defaultBatchSize, &timeoutMs, nil)
		if err != nil {
			common.Errorf("Canal 拉取失败: %s", err.Error())
			time.Sleep(time.Second) // 退避，防止错误风暴
			continue
		}
		if msg == nil || msg.Id == -1 || len(msg.Entries) == 0 {
			continue
		}

		if err := r.dispatch(msg.Entries); err != nil {
			// 处理失败不 Ack：Canal 会重新投递这一批
			common.Errorf("Canal 处理失败，不 Ack 等待重投: %s", err.Error())
			continue
		}
		if err := connector.Ack(msg.Id); err != nil {
			common.Errorf("Canal Ack 失败: %s", err.Error())
		}
	}
}

// dispatch 解析 binlog 事件并按 schema.table 分发给注册的 handler。
func (r *Runner) dispatch(entries []pbe.Entry) error {
	for i := range entries {
		// 取指针：Entry 内含 sync.Mutex，值拷贝会复制锁
		entry := &entries[i]

		// 关卡1：跳过事务开始/结束
		if entry.GetEntryType() == pbe.EntryType_TRANSACTIONBEGIN ||
			entry.GetEntryType() == pbe.EntryType_TRANSACTIONEND {
			continue
		}

		rowChange := &pbe.RowChange{}
		if err := proto.Unmarshal(entry.GetStoreValue(), rowChange); err != nil {
			common.Errorf("解析 binlog 事件错误: %s", err.Error())
			continue
		}

		schema := entry.GetHeader().GetSchemaName()
		table := entry.GetHeader().GetTableName()

		// 关卡2：只处理注册过的表
		handler, ok := r.handlers[schema+"."+table]
		if !ok {
			continue
		}

		// 关卡3：只处理 INSERT（发件箱场景就是「有新的领域事件落库」）
		if rowChange.GetEventType() != pbe.EventType_INSERT {
			continue
		}

		for _, rowData := range rowChange.GetRowDatas() {
			colMap := make(map[string]string, len(rowData.GetAfterColumns()))
			for _, col := range rowData.GetAfterColumns() {
				colMap[col.GetName()] = col.GetValue()
			}
			if err := handler.Handle(schema, table, colMap); err != nil {
				return err
			}
		}
	}
	return nil
}
