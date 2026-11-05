# 快速开始

## 1. 前置依赖

| 组件 | 默认地址 | 用途 |
|---|---|---|
| MySQL 8 | `localhost:3306` | 业务数据 |
| Redis | `localhost:6379` | 缓存 / 会话 / 红包池 / 在线路由 |
| Kafka | `localhost:9092` | 异步消息 |
| etcd | `localhost:2379` | 服务注册发现 |
| MinIO | `localhost:9000` | 头像/文件 |
| Canal | `localhost:11111` | binlog 订阅（offline 用） |

本地开发建议用 Docker Desktop 起一套；Canal 需要单独配置 MySQL 的 `binlog_format=ROW` 与授权账号。

## 2. 初始化数据库

```bash
mysql -uroot -p < IM.sql
```

`IM.sql` 建库建表；各 model 目录下还有单表 DDL（`service/*/model/**/*.sql`）可对照。

## 3. 修改配置

需要按本机情况调整的文件：

| 文件 | 关键字段 |
|---|---|
| `service/user/api/etc/user-api.yaml` | `DataSource`、`Redis`、`Cache`、`Kafka.Brokers`、`Etcd`、`Minio` |
| `service/user/rpc/etc/user.yaml` | `ListenOn`、`Etcd`、`DataSource`、`UserRedis` |
| `service/realtime/etc/realtime.yaml` | `WebSocket.Port/Path`、`Kafka`、`Etcd`、`UserRpc` |
| `service/offline/api/etc/offline-api.yaml` | `DataSource`、`Redis`、`Canal`、`Kafka` |
| `service/redpacket/api/etc/redpacket-api.yaml` | `DataSource`、`Redis`、`Kafka` |
| `service/gateway/etc/gateway.yaml` | `Redis`、`Etcd`、`Auth.WhiteList`、`Routes` |

配置里有本地开发用的明文口令（MySQL root、MinIO），仅供本机使用，不要提交到公开仓库的生产配置里。

## 4. 启动顺序

先起 rpc 和 api（它们会注册到 etcd），再起 realtime/offline/redpacket，最后起 gateway：

```bash
go run service/user/rpc/user.go
go run service/user/api/user.go
go run service/realtime/realtime.go
go run service/offline/api/offline.go
go run service/redpacket/api/redpacket.go
go run service/gateway/gateway.go
```

各进程监听端口见 [services.md](services.md)。

## 5. 验证

### 5.1 服务注册

```bash
etcdctl get --prefix /services
# 应能看到 /services/user.api/<addr>、/services/offline.api/<addr>、/services/redpacket.api/<addr>
# 以及 /services/realtimeservice/<addr>
```

### 5.2 注册 + 登录（走网关）

```bash
curl -X POST http://127.0.0.1:10010/api/user/users \
  -H 'Content-Type: application/json' \
  -d '{"account":"test@example.com","code":"123456","password":"123456","confirmPassword":"123456","nickname":"tester"}'
```

> 邮箱注册需要先调用 `GET /api/user/sendCaptcha?account=...` 拿验证码；
> **手机号**分支目前把验证码写死为 `123456`（开发桩，见 [roadmap.md](roadmap.md)）。

登录返回 `token` 与 `refreshToken`，后续请求带：

```bash
curl http://127.0.0.1:10010/api/user/users/<userId> \
  -H 'Authorization: Bearer <token>'
```

### 5.3 连接 WebSocket

```
ws://127.0.0.1:9101/ws/chat?userId=<userId>
```

连接后发送消息体（JSON）：

```json
{
  "sessionId": 1,
  "receiverId": 2,
  "senderId": 1,
  "type": 0,
  "sessionType": 0,
  "body": {"content": "hello"},
  "clientMessageId": "c-1"
}
```

服务端会补 `messageId` / `createdTime`，并推 `store-topic` 与 `message-topic`。

### 5.4 红包

```bash
POST http://127.0.0.1:10010/api/chat/redPacket/send
{
  "sessionId": 1, "senderId": 1, "sessionType": 0, "receiverId": 2,
  "body": {"totalAmount": "10.00", "totalCount": 5, "redPacketType": 0, "redPacketWrapperText": "恭喜发财"}
}
```

## 6. 常见问题

| 现象 | 原因 / 处理 |
|---|---|
| 网关返回 401，明明刚登录 | 检查 `gateway.yaml` 白名单是否覆盖了登录/注册路径（精确匹配） |
| 登录成功但接口都 401 | gateway 的 Redis 和 user-api 的 Redis 指向不一致 |
| 网关 503 | etcd 里没有该服务实例：服务没启动成功，或 `RegisterKey` 不一致 |
| 网关 502 | 上游地址不可达：检查 `Routes.Upstream` 与实例端口 |
| 消息发出去但对方收不到 | 确认 realtime 的两个 Kafka 消费者是否启动；确认接收者在线且连接在当前节点 |
| 群消息只推给发送者 | 检查 `GetGroupMemberIds` 的 SQL 是否为 `session_id` 条件 |
| 发红包 500 | 看日志里 `recover.go` 打出的"原因"；历史上有 `updateed_time` 列名错误 |
| 历史消息缺几条 | 秒级时间游标会导致同秒消息被跳过（见 roadmap） |
| Kafka 有消息但没落库 | 检查 offline 消费者是否启动、`ForceCommit` 配置、MySQL 连接 |

## 7. 调试技巧

- 业务错误：`Throw` 抛出后由 `RecoverMiddleWare` 记录到日志，带 `原因: <底层错误>`（本轮修复后）
- Kafka：`kq` 内部的 `logx` 日志级别由各服务 yaml 的 `Log.Level` 控制
- Redis：`redis-cli --scan --pattern 'session:*'`、`HGETALL user:1`、`ZRANGE session:1 0 -1`
