# go-infinitechat

基于 go-zero 的即时通讯 + 红包微服务项目：WebSocket 实时聊天、离线消息、网关动态路由、红包收发与过期退款。

## 技术栈

| 组件 | 用途 |
|---|---|
| go-zero / goctl | 微服务框架与代码生成（goctl 1.9.2） |
| MySQL | 业务数据落库（用户、消息、红包、余额） |
| Redis | 缓存、红包金额池、分布式锁、在线状态 |
| Kafka（go-queue/kq） | 异步解耦：消息落库/推送、红包领取/退款事件 |
| etcd | 服务注册与发现（网关动态路由） |
| MinIO | 文件/头像存储 |
| Canal | 监听 MySQL binlog，同步在线数据与消息落库 |
| WebSocket（gorilla） | 客户端长连接，消息实时推送 |

## 服务架构

| 服务 | 入口 | 职责 |
|---|---|---|
| gateway | [service/gateway/gateway.go](service/gateway/gateway.go) | 统一入口（:10010），Token 认证中间件，etcd 动态发现 + 路由转发 |
| user | [service/user/api](service/user/api)、[service/user/rpc](service/user/rpc) | 用户注册/登录、好友/群组、余额 |
| realtime | [service/realtime/realtime.go](service/realtime/realtime.go) | REST（:8102）+ WebSocket（:9101），消费 push-topic 实时推送 |
| offline | [service/offline/api/offline.go](service/offline/api/offline.go) | Canal 监听 binlog → Redis 在线/离线数据；消费 store-topic 落 MySQL 消息表 |
| redpacket | [service/redpacket/api/redpacket.go](service/redpacket/api/redpacket.go) | 红包发送/领取/查询，过期扫描调度 + 退款 |
| ai | [service/ai](service/ai) | 预留（开发中） |

## 目录结构

```
.
├── common/          # 公共组件：错误码、Kafka 推送器、etcd、中间件、事务封装、工具
└── service/         # 微服务，每个服务独立 model / api / rpc
    ├── gateway/
    ├── user/        # api + rpc
    ├── realtime/
    ├── offline/
    ├── redpacket/
    └── ai/
```

## 快速开始

依赖：MySQL、Redis、Kafka、etcd、MinIO、Canal（本地开发用 Docker Desktop 起中间件即可）。

1. 初始化数据库：执行根目录 `IM.sql`
2. 按需修改各服务配置：`service/*/etc/*.yaml`（MySQL/Redis/Kafka/etcd/MinIO 地址）
3. 依次启动：

```bash
go run service/user/rpc/user.go
go run service/user/api/user.go
go run service/realtime/realtime.go
go run service/offline/api/offline.go
go run service/redpacket/api/redpacket.go
go run service/gateway/gateway.go
```

> 注意：go-zero v1.10.3 不自动执行 api 文件中的 `validate` 标签，`validate + msg` 校验目前是摆设，待接入 `SetValidator`。

## 红包服务设计

### Redis 数据结构

| Key | 类型 | 说明 |
|---|---|---|
| `redpacket:{id}:pool` | List | 预分配金额池，每个元素一份金额（分），LPOP 原子领取 |
| `redpacket:{id}:records` | Hash | field=userId，value=领取金额（分），兼作防重复领取 |
| `redpacket-expire-zset` | ZSet | member=红包ID，score=过期时间戳（毫秒），过期扫描依据 |

pool/records 的 TTL 为 25 小时——比 24 小时过期时间多 1 小时，保证过期处理完成后缓存才失效。

### 发送流程

1. 参数校验：金额最多两位小数、总额 ≥ 数量（每份至少 1 分）、单个 ≤ 200 元
2. MySQL 事务：扣减发送者余额 → 创建红包记录 → 写余额变动日志
3. 金额预分配（普通红包等额 / 拼手气随机）→ 写 Redis 金额池 + 过期 ZSet
4. 推 Kafka：`message-store-topic`（offline 落库）、`message-push-topic`（realtime 推送，按 sessionId 分区保证同会话有序）、`redpacket-creation-topic`

### 领取流程（Lua 原子抢包）

`ExecuteReceiveLua` 单脚本原子完成：防重复（HEXISTS）→ 参数校验 → 过期判断（与 Redis TIME 对比）→ LPOP 取一份 → HSET 记录 → LLEN 判断是否领完。

| 返回码 | 含义 |
|---|---|
| 正数 | 领取成功，值为金额（分） |
| -1 | 已领取过（回读 Hash 返回原金额） |
| -2 | 已过期 |
| -3 | 金额池空（查库返回真实状态，见兜底设计） |

领取成功后推 `topic-redpacket-receive`；若本次恰好领完，再推 `topic-redpacket-completed`。

### 异步落账（Kafka 消费端）

- **ReceiveConsumer**：幂等检查（同红包同用户已有记录则跳过）→ 事务三步骤：插入领取记录 + 增加余额 + 写余额变动日志
- **CompletedConsumer**：红包状态置为已领完 + 清理 Redis 缓存
- **ExpirationConsumer**：状态幂等校验（已过期则跳过）→ 计算剩余金额 → 事务（退款给发送者 + 写日志 + 状态置为已过期）→ 清理缓存

### 过期退款（扫描调度器）

`expiration_dispatcher` 每秒扫描：Redis 分布式锁（`lock:expiration-dispatcher`）保证多实例下单实例执行 → Lua 按 score 批量扫出过期红包 ID（限批次与时间预算）→ 推 `redpacket-expiration-topic`。推送失败时把 ID 放回 ZSet（score=当前时间），下轮重新扫出重试。

### 兜底设计

**过期时间兜底（`getRedPacketExpireMs`）**

优先取 `redpacket-expire-zset` 的 score（发送红包时写入）；缺失时用 DB `created_time + 24h` 推导。

- 触发场景：过期摘除后的迟到请求 / 发送流程半初始化崩溃 / Redis 数据意外丢失
- 边界：不兜底"红包不存在"——FindOne 失败传 0，由 Lua 参数校验分支报错

**金额池为空兜底（EmptyPool / -3）**

Redis 中 poolKey 不存在时查库，按真实状态返回"已领取完"/"已过期"；其余状态走 default 分支返回"暂时无法领取"，用于应对 Redis 实时真相与 MySQL 异步追账之间的状态同步时间窗。

### 已知问题与规划

- **领取成功但 Kafka 推送失败**：LPOP 已抢到金额，但消费端事务三步骤不会执行，且无重试——金额只存在于 Redis，TTL 到期后永久丢失。根治方案：事务性发件箱（Transactional Outbox），把业务数据与要发的 Kafka 事件放进同一个 MySQL 事务，后台任务推送成功才删除
- **过期退款推送失败闭环**：红包 ID 从 ZSet 摘除后，若 Kafka 推送失败且放回 ZSet 也失败，退款永久丢失（当前只能靠日志排查）
- **剩余金额计算全量 LRANGE**：大拼手气包会阻塞 Redis；更优做法是独立金额 key，每次 LPOP 后 DECRBY，剩余金额 O(1) 读取
- **热数据分支补差额**：MySQL 边界用 hotBoundary（7 天前）依赖 Redis 严格保留 7 天的假设，Redis 不满 7 天时中间会留空洞（设计待定）

## 错误处理约定（方案 A，2026-09 确立）

所有错误最终都汇到 `httpx.ErrorCtx` → 统一错误处理器（`service/user/api/user.go` 的 `SetErrorHandlerCtx`），响应格式统一。logic 层只有两种合法写法：

| 场景 | 写法 | 示例 |
|---|---|---|
| 有底层 err（系统错误，code >= 50000） | 记日志 + `return "", common.WrapError(common.SystemError, err)` | Redis 连接失败、发邮件失败、生成验证码失败 |
| 无底层 err（业务判断，code < 50000） | `common.Throw(common.PhoneEmailError)` | 账号格式错误 |

规则细节：

- logic 禁止裸传原始 error——未包装的错误会触发"未包装错误"日志并被 500 兜底，业务错误被误判成故障
- `Throw` 抛出的 panic 由 `common/middleware` 的 `RecoverMiddleWare` 接住并转交 `ErrorCtx`；该中间件是必需件，删除后 Throw 会变成"空响应体的 500"（go-zero 自带 RecoverHandler 只记日志不写 body）
- 业务错误码 → HTTP 200 + `{code, message}`；系统错误码 → HTTP 500 + 脱敏文案
- **panic 只能用在 HTTP 请求路径（同步流程）**：异步 goroutine 里的 panic 不会被请求中间件的 recover 接住，会直接崩掉进程，异步场景必须 `return` error
