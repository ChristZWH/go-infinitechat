# 服务说明

本文按服务列出职责、关键文件与依赖。所有服务共用 `common/` 下的错误码、日志、Kafka Pusher、etcd 注册、事务封装。

## gateway（:10010）

**职责**：唯一对外入口。CORS → 鉴权 → 反向代理；`etcd://` 上游从 etcd 订阅实例并轮询。

| 文件 | 说明 |
|---|---|
| `gateway.go` | 装配：配置、Redis、etcd 客户端、中间件链、反向代理 |
| `internal/middleware/auth.go` | 白名单 + `access_token:<token>` 查 Redis，注入 `X-User-Id` |
| `internal/middleware/cors.go` | 允许来源/方法/头，`OPTIONS` 直接 204 |
| `internal/handler/proxy.go` | 路由表构建（静态 http:// 或 etcd://）、按前缀匹配、502/503 兜底 |
| `internal/discovery/resolver.go` | etcd 全量加载 + watch 重载 + 轮询 |

配置：`etc/gateway.yaml`（白名单、CORS、路由、超时）。

**注意**：白名单是**精确匹配**；handler 里注册的路径变了，这里必须同步改，否则登录会被 401。

## user（api :8104 + rpc :8105）

**职责**：账号、好友、群组、余额、会话、通知的全部业务写操作；rpc 供 realtime/offline 调用做校验与批量查询。

### user.api

| 目录 | 内容 |
|---|---|
| `internal/logic/user/` | 注册、登录（密码/验证码）、登出、刷新 token、改密、头像、余额 |
| `internal/logic/contact/` | 搜索、好友列表/详情、申请列表、发申请、处理申请、拉黑、删好友 |
| `internal/logic/group/` | 建群、邀请、踢人、退群、群列表、成员分页 |
| `internal/service/` | 业务核心：`friend_service`、`group_service`、`apply_friend_service`、`balance_service`、`notification_service`、`ws_server_locator` |
| `user.go` | 统一错误/成功响应、`RecoverMiddleWare`、etcd 注册 |

主要接口（前缀 `/api/user`、`/api/contact`、`/api/group`）：

```
POST   /api/user/users                        注册
POST   /api/user/login                        登录（type=password|code）
GET    /api/user/sendCaptcha                  发送验证码
GET    /api/user/users/:userId                用户信息
GET    /api/user/users/:userId/balance        余额
GET    /api/user/users/:userId/balance/records 余额明细
POST   /api/contact/users/:userId/friend/:receiveUserId   发好友申请
GET    /api/contact/users/:userId/friend      好友列表
GET    /api/contact/users/:userId/apply       申请列表
PUT    /api/contact/users/:userId/block/:receiveUserId    拉黑
POST   /api/group/                            建群
GET    /api/group/:sessionId/members          群成员（分页）
GET    /api/group/users/:userId               我加入的群
```

### user.rpc

| 方法 | 用途 |
|---|---|
| `ValidateSingleMessage` | 单聊发送前校验（发送者/接收者状态、好友关系、拉黑） |
| `ValidateGroupMessage` | 群聊发送前校验（是否群成员） |
| `GetGroupMemberIds` | 群成员 ID 列表（realtime 群发用） |
| `GetUserSessionRole` | 用户在会话中的角色 |
| `BatchGetUserInfos` | 批量取昵称/头像（offline、redpacket 展示用） |
| `GetUserInfo` | 单个用户信息 |

**注意**：`GetGroupMemberIds` 的 SQL 条件必须是 `session_id`（曾误写为 `user_id`，导致群消息只推给发送者，已有回归测试）。

## realtime（REST :8102 / WS :9101）

**职责**：WebSocket 长连接管理、消息推送、通知推送。

| 文件 | 说明 |
|---|---|
| `realtime.go` | 启动 REST + WS + 两个 Kafka 消费者 |
| `internal/websocket/ws_server.go` | 握手、读循环、消息校验、双写 store/message topic、路由写 Redis |
| `internal/channelmgr/channel_manager.go` | 双向映射（uid ↔ 连接），写操作加锁，踢旧连接 |
| `internal/consumer/message_consumer.go` | 消费 `message-topic`，单聊/群聊分发推送 |
| `internal/consumer/notification_consumer.go` | 消费 `system-notification-topic`，在线直推、离线转 `store-notification-topic` |
| `internal/svc/servicecontext.go` | Redis、三个 Pusher、etcd 注册、UserRpc 客户端 |

**注意**：
- 消费者必须显式启动（`realtime.go` 里曾只有一个 `// go` 占位）
- 通知落库 Pusher 必须指向 `store-notification-topic`，不能指向自己消费的 topic
- 单聊消息缺少 `receiverId` 时要在解引用前拦下（kq 消费协程没有 recover）

## offline（:8101）

**职责**：消息落库、历史/离线消息查询、系统通知、Canal 同步 Redis。

| 文件 | 说明 |
|---|---|
| `offline.go` | 启动 HTTP、两个落库消费者、Canal runner |
| `internal/consumer/message_store_consumer.go` | `store-topic` → MySQL `message`（1062 幂等） |
| `internal/consumer/notification_store_consumer.go` | `store-notification-topic` → MySQL `system_notification` |
| `internal/canal/cannal_client.go` | 解析 binlog → 写 Redis 四件套（消息 ZSet、活跃榜、会话快照、未读数） |
| `internal/logic/offlinedata/` | 历史消息、离线消息、离线数据 |
| `internal/logic/systemnotification/` | 未读数、未读列表、标记已读/全部已读 |

**注意**：`kq.KqConf.ForceCommit` 默认 `true`（失败也提交 offset），落库类消费者必须显式设为 `false`。

## redpacket（:8888）

**职责**：发红包（扣款 + 建池）、Lua 抢包、领取/领完/过期事件消费、过期扫描与退款。

| 文件 | 说明 |
|---|---|
| `internal/logic/sendredpacketlogic.go` | 事务（扣款 + 建红包 + 写流水）→ 金额分配 → Redis 池与 ZSet → 推消息 |
| `internal/logic/receiveredpacketlogic.go` | 执行 Lua → 处理 -1/-2/-3 分支 → 推领取事件 |
| `internal/service/redpacket_service.go` | Lua 执行、金额转换、余额增减、缓存清理 |
| `internal/script/lua_scripts.go` | 抢包 / 计算余额 / 过期扫描脚本 |
| `internal/consumer/redpacket_consumer.go` | 领取入账、领完流转、过期退款 |
| `internal/schedular/expiration_dispatcher.go` | 定时扫描 ZSet，投递过期事件 |
| `internal/algorithm/redpacket_algorithm.go` | 普通/拼手气金额拆分（有单测） |

详见 [redpacket.md](redpacket.md)。

## common

| 包 | 内容 |
|---|---|
| `common/common` | 错误码、响应结构、zap 日志、validator、Throw/WrapError |
| `common/middleware` | `RecoverMiddleWare`：接住 Throw，交给统一错误处理器 |
| `common/kafka` | `PusherManager`：按 topic 复用 Pusher，Push / KPush |
| `common/etcd` | `RegisterHTTPService`：租约 + KeepAlive + 主动注销 |
| `common/model/txctx` | 事务 session 注入 ctx，嵌套复用外层事务 |
| `common/model/dto` | `MessageRequest/Response`、`PageResponse` 分页 |
| `common/model/constants` | Kafka topic、Redis key、好友缓存 key 构造 |
| `common/utils` | 雪花 ID |
