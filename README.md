# go-infinitechat

基于 **go-zero** 的即时通讯（IM）微服务练手项目：WebSocket 实时聊天、离线消息、系统通知、网关动态路由、红包收发与过期退款。

> 面向学习：把"网关 / 服务发现 / 实时推送 / 离线存储 / 分布式事务 / 抢红包"这些典型场景串成一个能跑起来的系统。
> 目前是**本地开发环境**（MySQL / Redis / Kafka / etcd / MinIO / Canal 全在本机），尚未做生产化加固，已知限制见 [docs/roadmap.md](docs/roadmap.md)。

## 功能

| 模块 | 能力 |
|---|---|
| 用户 | 手机号/邮箱注册登录、JWT + Refresh Token、单点登录（踢旧会话）、资料与头像、余额与明细 |
| 好友 | 搜索用户、申请/通过/拒绝、拉黑/解除、删除好友、好友列表分页与关键字过滤 |
| 群组 | 建群、邀请、踢人、退群、成员分页、群列表 |
| 消息 | WebSocket 收发、单聊/群聊、回复、红包消息、历史消息、离线消息、未读数 |
| 通知 | 好友申请 / 新会话 / 群变更 / 系统通知，在线直推、离线落库 |
| 红包 | 普通红包 + 拼手气、Lua 原子抢包、领完/过期状态流转、过期自动退款 |
| 网关 | 统一入口、Token 校验、etcd 服务发现 + 轮询负载均衡、CORS |

## 技术栈

| 组件 | 用途 |
|---|---|
| go-zero 1.10.3 / goctl 1.9.2 | 微服务框架与代码生成 |
| MySQL 8 | 用户、好友、群、消息、红包、余额落库 |
| Redis | 缓存、红包金额池、JWT 会话、在线路由、热数据 ZSet |
| Kafka（go-queue/kq） | 消息落库、实时推送、通知、红包事件 |
| etcd | 服务注册与发现 |
| MinIO | 头像/文件对象存储 |
| Canal | 监听 MySQL binlog，同步离线消息与未读数 |
| WebSocket（gorilla） | 客户端长连接 |

Go 版本：`go 1.26.2`（见 `go.mod`）。

## 架构

```
                    ┌─────────────────────────────┐
   Web / App ──────▶│  gateway :10010             │
      HTTP          │  CORS → Auth(Redis token)   │
                    │  → 反向代理 / 负载均衡       │
                    └──────┬───────────┬──────────┘
                           │           │
        ┌──────────────────┼───────────┼──────────────────┐
        ▼                  ▼           ▼                  ▼
  user.api :8104     offline.api   redpacket.api     ai (外部 :8886)
        │              :8101          :8888
        ▼                │              │
  user.rpc :8105         │              │
        │                ▼              ▼
        └──────────▶ MySQL / Redis ◀────┘
                           ▲
                           │ binlog（Canal）
                    ┌──────┴───────┐
   WebSocket ──────▶│  realtime    │ REST :8102 / WS :9101 (/ws/chat)
      :9101         └──────┬───────┘
                           ▼
                   Kafka: message-topic / system-notification-topic
```

消息主链路（单聊）：

1. 客户端通过 WebSocket 发消息 → `realtime` 校验 → 推 `store-topic`（落库）与 `message-topic`（推送），key 用 sessionId 保序
2. `offline` 消费 `store-topic` → 写 MySQL `message` 表
3. Canal 监听 binlog → 写 Redis（离线 ZSet、会话摘要、未读数）
4. `realtime` 消费 `message-topic` → 按接收者在线的节点推送；不在线则留给离线接口拉取

细节见 [docs/architecture.md](docs/architecture.md)。

## 服务与端口

| 服务 | 端口 | 入口 |
|---|---|---|
| gateway | 10010 | [service/gateway/gateway.go](service/gateway/gateway.go) |
| user.api | 8104 | [service/user/api/user.go](service/user/api/user.go) |
| user.rpc | 8105 | [service/user/rpc/user.go](service/user/rpc/user.go) |
| realtime | 8102（REST）/ 9101（WebSocket） | [service/realtime/realtime.go](service/realtime/realtime.go) |
| offline.api | 8101 | [service/offline/api/offline.go](service/offline/api/offline.go) |
| redpacket.api | 8888 | [service/redpacket/api/redpacket.go](service/redpacket/api/redpacket.go) |

## 目录结构

```
.
├── common/                  # 公共组件
│   ├── common/              # 错误码、响应封装、日志、validator
│   ├── middleware/          # Recover（Throw 的接住方）
│   ├── kafka/               # Pusher 管理（按 topic 复用）
│   ├── etcd/                # HTTP 服务注册（租约 + KeepAlive）
│   ├── model/txctx/         # 事务 session 注入 context
│   ├── model/dto/           # 消息体、分页 DTO
│   └── utils/               # 雪花 ID 等
├── service/
│   ├── gateway/             # 网关：鉴权中间件 + 反代 + etcd 发现
│   ├── user/api|rpc/        # 用户侧 API 与 RPC
│   ├── realtime/            # WebSocket + 推送消费者
│   ├── offline/             # 离线/历史消息 + Canal + 落库消费者
│   └── redpacket/           # 红包：Lua 抢包、事件消费、过期扫描
├── docs/                    # 项目文档
└── IM.sql                   # 建库建表脚本
```

## 快速开始

```bash
# 1. 起中间件：MySQL / Redis / Kafka / etcd / MinIO / Canal
# 2. 初始化数据库
mysql -uroot -p < IM.sql
# 3. 按需改配置中的连接信息
#    service/*/etc/*.yaml
# 4. 启动（顺序：rpc → api → realtime → offline → redpacket → gateway）
go run service/user/rpc/user.go
go run service/user/api/user.go
go run service/realtime/realtime.go
go run service/offline/api/offline.go
go run service/redpacket/api/redpacket.go
go run service/gateway/gateway.go
```

完整步骤（含中间件 docker-compose 建议、验证方式、常见问题）见 [docs/getting-started.md](docs/getting-started.md)。

## 测试

```bash
go test ./...                 # 单元测试（不需要任何外部依赖）
go test -race ./...           # 竞态检测
go vet ./...
```

真实库结构回归测试需要 MySQL（默认跳过）：

```powershell
$env:TEST_MYSQL_DSN = "root:password@tcp(127.0.0.1:3306)/infiniteChat?charset=utf8mb4&parseTime=True&loc=Local"
go test ./service/user/model/ -v
```

说明见 [docs/testing.md](docs/testing.md)。

## 文档

| 文档 | 内容 |
|---|---|
| [docs/README.md](docs/README.md) | 文档索引 |
| [docs/architecture.md](docs/architecture.md) | 服务拓扑、消息流转、存储结构、Kafka topic |
| [docs/services.md](docs/services.md) | 每个服务的职责、关键文件、依赖 |
| [docs/getting-started.md](docs/getting-started.md) | 环境准备、启动顺序、联调验证、排错 |
| [docs/error-handling.md](docs/error-handling.md) | 错误码体系与 Throw/WrapError 约定 |
| [docs/auth.md](docs/auth.md) | 身份与鉴权：网关注入、路径校验、ctx 身份 |
| [docs/redpacket.md](docs/redpacket.md) | 红包设计：Redis 结构、Lua、状态机、退款 |
| [docs/testing.md](docs/testing.md) | 测试分层与如何运行 |
| [docs/roadmap.md](docs/roadmap.md) | 已知限制与待办（按优先级） |
| [docs/fixes-2026-11-05.md](docs/fixes-2026-11-05.md) | 本轮代码审查后的修复记录与验证 |
| [docs/code-review-2026-11-05.md](docs/code-review-2026-11-05.md) | 全量代码审查报告（未修项仍在此跟踪） |

## 当前状态

- ✅ 用户/好友/群组/消息/通知/红包主链路已打通
- ✅ 单元测试覆盖：红包算法、分页、错误码、好友缓存 key、连接管理、JWT、网关路由与白名单、身份中间件
- ✅ 越权收口：网关校验 `/users/{id}` 与登录身份一致；服务侧把身份解析进 context，user.api / offline / redpacket 全部以登录态为准，群成员与消息读取增加会话归属校验（详见 [docs/auth.md](docs/auth.md)）
- ⚠️ user.rpc 的内部调用仍信任传入的 senderId（见 [docs/roadmap.md](docs/roadmap.md) 第 1 节）
- ⚠️ 配置里有本地开发用的明文口令，勿直接用于公网

## 说明

本项目为个人学习用途，代码与设计持续演进；欢迎对照文档阅读源码。
