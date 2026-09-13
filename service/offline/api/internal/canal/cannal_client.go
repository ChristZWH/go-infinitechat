package canal

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go-infinitechat/common/common"
	"go-infinitechat/common/model/constants"
	"go-infinitechat/service/offline/api/internal/config"
	"go-infinitechat/service/user/rpc/user"
	"go-infinitechat/service/user/rpc/userrpc"

	"github.com/gogo/protobuf/proto"
	"github.com/withlin/canal-go/client"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	pbe "github.com/withlin/canal-go/protocol/entry"
)

var monitorTables = map[string]bool{
	"InfiniteChat.message": true,
}

const (
	onlineUserHashKey = "wsServerUri"
	batchSize         = 1000
	idleCheckInterval = 5 * time.Second
)

// CanalClient
type CanalRunner struct {
	config  config.CanalConf
	rds     *redis.Redis
	sqlConn sqlx.SqlConn
	userRpc userrpc.UserRpc
}

func NewCanalRunner(canalConfig config.CanalConf, rds *redis.Redis, conn sqlx.SqlConn, userRpc userrpc.UserRpc) *CanalRunner {
	return &CanalRunner{
		config:  canalConfig,
		rds:     rds,
		sqlConn: conn,
		userRpc: userRpc,
	}
}

func (cr *CanalRunner) Start(ctx context.Context) {
	connector := client.NewSimpleCanalConnector(
		cr.config.Host,
		cr.config.Port,
		cr.config.Username,
		cr.config.Password,
		cr.config.Destination,
		60000,
		0,
	)

	if err := connector.Connect(); err != nil {
		common.Errorf("Canal 连接失败: %s", err.Error())
		return
	}
	defer connector.DisConnection()
	common.Infof("Canal 连接成功，destination=%s", cr.config.Destination)

	if err := connector.Subscribe(cr.config.Filter); err != nil {
		common.Errorf("Canal 订阅失败: %s", err.Error())
		return
	}

	for {
		select {
		case <-ctx.Done():
			common.Info("Canal 客户端已停止")
			return
		default:
		}

		msg, err := connector.Get(int32(batchSize), nil, nil)
		if err != nil {
			common.Errorf("Canal 拉取消息失败: %s", err.Error())
			time.Sleep(idleCheckInterval)
			continue
		}

		// 没有新数据
		if msg.Id == -1 || len(msg.Entries) == 0 {
			time.Sleep(idleCheckInterval)
			continue
		}
		// 有，处理新数据
		cr.handleMessage(msg.Entries)
		// 处理完没有 Ack —— Canal 会反复投递同一批
		// 通知 Canal 消费进度。否则服务端的游标不前进，同一批 binlog 反复推送，消息会重复写 Redis 无数遍
		connector.Ack(msg.Id)
	}
}

func (cr *CanalRunner) handleMessage(entries []pbe.Entry) {
	for i := range entries {
		// 取指针遍历：Entry 内含 sync.Mutex，值拷贝会复制锁
		// 取指针后全称操作同一个对象，同一把锁
		entry := &entries[i]
		// 关卡1 ：跳过事务开始，事务结束相关内容
		if entry.GetEntryType() == pbe.EntryType_TRANSACTIONBEGIN ||
			entry.GetEntryType() == pbe.EntryType_TRANSACTIONEND {
			continue
		}

		rowChange := &pbe.RowChange{}
		// 一个 Entry（信封） 包含：Header(元信息、库名、表名、事件类型TRANSACTIONBEGIN...)；StoreValue（信封里的内容）一串字符串（[]byte）
		if err := proto.Unmarshal(entry.GetStoreValue(), rowChange); err != nil {
			common.Errorf("解析binlog事件错误: %s", err.Error())
			continue
		}

		// 获取 数据库名 和表名
		schemaName := entry.GetHeader().GetSchemaName()
		tableName := entry.GetHeader().GetTableName()
		fullTableName := schemaName + "." + tableName

		// 关卡2 ：非 InfiniteChat.message 表 不处理
		if !monitorTables[fullTableName] {
			continue
		}

		logx.Infof("======> binlog name[%s,%s], eventType: %s", schemaName, tableName, rowChange.GetEventType())

		// 关卡3 ：非 INSERT 变更不进行处理
		if rowChange.GetEventType() != pbe.EventType_INSERT {
			continue
		}

		// 一个 Entry 内部可能包含多个 RowData
		for _, rowData := range rowChange.GetRowDatas() {
			// 变更【后】这一行长什么样（INSERT 时 = 新插入的完整行），INSERT只有AfterColumns
			cr.handleInsert(rowData.GetAfterColumns())
		}
	}
}

func (cr *CanalRunner) handleInsert(columns []*pbe.Column) {
	colMap := make(map[string]string, len(columns))
	for _, col := range columns {
		colMap[col.GetName()] = col.GetValue()
	}
	logx.Infof("表名：message，数据：%v", colMap)

	// 1. 构建消息对象
	msgResp := buildMessageFromMap(colMap)

	// 2. 存储到 Redis
	cr.storeMessageToRedis(msgResp)
}

type MessageRedisResponse struct {
	SessionId   int64        `json:"sessionId"`
	SenderId    int64        `json:"senderId"`
	MessageId   int64        `json:"messageId"`
	Type        int          `json:"type"`
	SessionType int          `json:"sessionType"`
	CreatedTime string       `json:"createdTime"`
	Body        *MessageBody `json:"body"`
	Name        string       `json:"name"`
	Avatar      string       `json:"avatar"`
}

type MessageBody struct {
	Content              string `json:"content,omitempty"`
	ReplyId              *int64 `json:"replyId,omitempty"`
	RedPacketId          string `json:"redPacketId,omitempty"`
	RedPacketWrapperText string `json:"redPacketWrapperText,omitempty"`
}

func buildMessageFromMap(colMap map[string]string) *MessageRedisResponse {
	resp := &MessageRedisResponse{}
	resp.SessionId, _ = strconv.ParseInt(colMap["session_id"], 10, 64)
	resp.SenderId, _ = strconv.ParseInt(colMap["sender_id"], 10, 64)
	resp.MessageId, _ = strconv.ParseInt(colMap["message_id"], 10, 64)
	resp.Type, _ = strconv.Atoi(colMap["type"])
	resp.SessionType, _ = strconv.Atoi(colMap["session_type"])
	resp.CreatedTime = colMap["created_time"]

	if resp.Type == 3 {
		body := &MessageBody{}
		_ = json.Unmarshal([]byte(colMap["content"]), body)
		resp.Body = body
	} else {
		body := &MessageBody{Content: colMap["content"]}
		if v := colMap["reply_id"]; v != "" {
			replyId, _ := strconv.ParseInt(v, 10, 64)
			body.ReplyId = &replyId
		}
		resp.Body = body
	}
	return resp
}

func (cr *CanalRunner) storeMessageToRedis(msg *MessageRedisResponse) {
	redisKey := fmt.Sprintf("session:%d", msg.SessionId)

	// 1. 查询成员会话有谁
	var receiverIds []int64
	_ = cr.sqlConn.QueryRowsCtx(context.Background(), &receiverIds, "SELECT user_id FROM user_session WHERE session_id = ?", msg.SessionId)
	if len(receiverIds) == 0 {
		return
	}

	// 2. RPC 调用user服务：补充昵称/头像
	userInfoMap := cr.getUserInfoMap(receiverIds)
	if info, ok := userInfoMap[msg.SenderId]; ok {
		msg.Name = info.Nickname
		msg.Avatar = info.Avatar
	}

	messageJson, _ := json.Marshal(msg)

	loc, _ := time.LoadLocation("Asia/Shanghai")
	t, err := time.ParseInLocation("2006-01-02 15:04:05", msg.CreatedTime, loc)
	if err != nil {
		logx.Errorf("解析消息时间失败: %s", err.Error())
		return
	}
	score := t.UnixMilli()

	// 3. 获取离线用户 ID ，从 key = wsServerUri 中获取，field 为 uid 的值
	offlineUserIds := cr.getOfflineUserIds(receiverIds)

	// 4.  写入 Redis 四件套
	// 4.1 存储消息到会话 ZSet + 清理 7 天前旧消息
	_, _ = cr.rds.Zadd(redisKey, score, string(messageJson))
	cr.rds.Zremrangebyscore(redisKey, 0, time.Now().UnixMilli()-constants.SevenDaysMillis)

	// 4.2 更新活跃会话（目前有问题，此会话为全局活跃榜，应更正为当前用户活跃榜单，用户聊天列表排序）
	_, _ = cr.rds.Zadd("session:active", score, strconv.FormatInt(msg.SessionId, 10))

	// 4.3 为离线用户存储会话快照和未读计数
	for _, receiverId := range receiverIds {
		// 因为离线快照 + 未读计数的语义是："你没读到的消息"，不针对发送者
		if receiverId == msg.SenderId {
			continue
		}
		if !offlineUserIds[receiverId] {
			continue
		}

		objectKey := strconv.FormatInt(msg.SessionId, 10)

		lastMsgContent := ""
		if msg.Type == 3 && msg.Body != nil {
			lastMsgContent = "[红包] " + msg.Body.RedPacketWrapperText
		} else if msg.Body != nil {
			lastMsgContent = msg.Body.Content
		}

		sessionFields, _ := json.Marshal(map[string]string{
			"type":           strconv.Itoa(msg.Type),
			"sessionType":    strconv.Itoa(msg.SessionType),
			"senderId":       strconv.FormatInt(msg.SenderId, 10),
			"avatar":         msg.Avatar,
			"name":           msg.Name,
			"lastMsgContent": lastMsgContent,
			"lastMsgTime":    msg.CreatedTime,
		})
		// 最新一次会话快照
		_ = cr.rds.Hset(fmt.Sprintf("user:%d", receiverId), objectKey, string(sessionFields))

		// 4.4 未读计数 Hash
		_, _ = cr.rds.Hincrby(fmt.Sprintf("user:%d:count", receiverId), objectKey, 1)
	}
}

func (cr *CanalRunner) getUserInfoMap(userIds []int64) map[int64]*user.BatchUserInfoItem {
	result := make(map[int64]*user.BatchUserInfoItem)
	resp, err := cr.userRpc.BatchGetUserInfos(context.Background(), &user.BatchGetUserInfosReq{UserIds: userIds})
	if err != nil || resp == nil {
		return result
	}
	// GetUsers 拿到整个数组
	for _, item := range resp.GetUsers() {
		result[item.UserId] = item
	}
	return result
}

// 从 key = wsServerUri 中获取，field 为 uid 的值
func (cr *CanalRunner) getOfflineUserIds(userIds []int64) map[int64]bool {
	offline := make(map[int64]bool)
	for _, uid := range userIds {
		exists, _ := cr.rds.Hexists(onlineUserHashKey, strconv.FormatInt(uid, 10))
		if !exists {
			offline[uid] = true
		}
	}
	return offline
}
