# 待办与已知限制

按"上线前必须解决"到"可以慢慢做"排序。来源：2026-11-05 代码审查（见 [code-review-2026-11-05.md](code-review-2026-11-05.md)）+ 日常开发中的 TODO。

## P0 上线前必须解决

### 1. 身份透传与越权

**现状**：网关鉴权后把身份写入 `X-User-Id`，但全仓库没有任何服务读取它；所有接口的 userId 来自 URL path 或请求体。

**影响**：任意登录用户可删除他人好友、拉黑、改密码/头像、查他人余额与明细、踢人、通过他人好友申请。

**方案**：
1. gateway 保持注入 `X-User-Id`（并清理客户端自带的同名头）
2. 各 api 服务加中间件：解析 `X-User-Id` 存入 ctx；同时在路由入口校验"路径里的 userId 必须等于 ctx 里的 userId"（除管理员/机器人场景）
3. logic 层统一从 ctx 取身份，不再信任 `req.UserId`
4. 给 30+ 个接口补回归测试（可用同一个 fake 中间件批量覆盖）

### 2. 手机验证码写死 `123456`

**现状**：`loginlogic.go` / `registerlogic.go` 的手机号分支直接比对常量 `123456`。

**方案**：手机与邮箱共用 Redis 验证码流程（`sendCaptcha` 写入、登录/注册校验后删除、5 分钟 TTL）；短信通道未接入时用配置开关 `AllowFixedCaptcha`，默认仅在本地 profile 打开。

### 3. offline 服务无鉴权

**现状**：只挂了 `RecoverMiddleWare`；历史/离线消息接口只接收 `sessionId`，请求体里没有 userId。

**方案**：接口增加身份来源，服务端校验 `user_session` 中该用户确实属于该会话；网关侧把 offline 路由纳入鉴权。

### 4. 红包身份与"不能领自己的包"

**现状**：领取接口的 `userId` 来自请求体；Lua 只做防重复领取。

**方案**：身份取鉴权上下文；领取前校验 `rp.SenderId != userId`。

## P1 数据一致性 / 可靠性

| 项 | 说明 | 建议 |
|---|---|---|
| 发红包幂等 | `clientMessageId` / `PreventDuplicatePrefix` 已定义未使用，重复提交会重复扣款 | 用 `clientMessageId` 做 Redis SETNX 幂等键，或数据库唯一约束 |
| LPOP 后 Kafka 失败 | 金额已从池里拿走，事件没发出 → 钱既不入账也不回池 | Lua 补偿（HDEL + LPUSH 回池）；根治用事务性发件箱 |
| `InitRedisPool` 失败 | 事务已提交、钱已扣，但红包不可领也扫不到 | 失败时补偿退款 + 定时对账「status=0 且不在 ZSet」 |
| 过期扫描先 ZREM 再投递 | 进程崩溃 / 投递失败 → 该红包永远不再被扫描 | 消费成功后再摘；或先写 MySQL 任务表再摘 |
| 扫描锁 TTL 1s < 单轮预算 5s | 多实例互斥中途失效 | TTL ≥ 预算 + 余量，或加续期 |
| 通过好友申请非原子 | 先把申请置为通过（独立提交），再建好友关系 | 两步合进同一事务 |
| 删好友不清 model 缓存 | `Tx` 方法走裸 SQL，不触发 go-zero 缓存失效 | 提交后显式删本地 cache key |
| etcd 续租 ctx 被提前取消 | realtime 注册用的 ctx 带 30s 超时且 `defer cancel()` | 用不取消的 ctx；KeepAlive 断开时重新注册 |
| 通知在线判定 | 用 `Hexists(wsServerUri)` 判断在线，而登录时就写入 → 永远判在线 → 通知既不推也不落库 | 查真实连接，或给路由 key 加 TTL |
| 通知推送载荷 | 推的是裸 `messageId` 而非消息 JSON | 推 `val` |
| 跨节点推送 | `message-topic` 全局共享消费组，接收者不在本节点就丢 | 按 sessionId hash 分区，或节点间转发 |
| Kafka 消费失败吞掉 | realtime 消费失败也 `return nil`，offset 照常提交 | 失败返回 error + 有限重试 + 死信 |
| Canal 用 `Get` 而非 `GetWithOutAck` | 位点在处理前就前进，Redis 错误被 `_ =` 吞掉 | 改 `GetWithOutAck`，整批成功再 Ack |

## P2 性能与工程质量

| 项 | 说明 |
|---|---|
| 好友列表 N+1 | 先全量拉关系，循环内每人两次查询，分页在内存做 → 改批量 `IN` + SQL 分页 |
| Redis 路由无 TTL | 进程被 kill 后残留，`Hget`+`Hdel` 非原子 → 加 TTL + Lua 原子摘除 |
| `LRANGE 0 -1` 全量累加 | 大拼手气包会阻塞 Redis → 独立 remain key + `DECRBY` |
| `records` Hash 不过期 | 对尚不存在的 key 做 `EXPIRE` 无效 → 首次写入后设置 TTL |
| 未读数无上限 | `unreadOfflineCount` 可传 2^31-1 → 上限截断 |
| 分页 pageSize | 部分 logic 直接 `int32()` 强转，超大值全表扫描 / 溢出 → 统一 `Normalize()` |
| 无服务端心跳 | 只读不发的客户端每 60s 被踢；死连接要等满 60s 释放 → 加 ticker Ping |
| 密码存储 | MD5 + 固定盐 → bcrypt / argon2id |
| 口令进日志 | 登录日志打印了 token / refreshToken → 脱敏 |
| 密钥硬编码 | JWT 密钥在源码、MySQL root 密码在受跟踪配置 → 环境变量 / 未跟踪文件 |
| 双日志体系 | zap（Debug）与 logx（error）并存 → 统一到一个 logger |
| 网关细节 | `Next()` 用写锁做读、每请求打 Info 日志、无 `ReadHeaderTimeout`、无优雅关闭 |
| 雪花 ID | `WorkerID` 固定为 1，多实例可能撞；`NewNode` 失败导致 nil 解引用 |
| 死常量 | redpacket 的 `message-store-topic` / `message-push-topic` 与实际 topic 不一致且未使用 |
| 历史消息游标 | 秒级 `datetime` 游标会跳过同秒消息 → 复合游标 `(created_time, message_id)` |
| 热/冷边界 | 补差额用 `hotBoundary` 而非请求游标，Redis 不满 7 天会出现空洞 |

## P3 测试与文档

- [ ] handler → logic → MySQL 的端到端测试（sqlmock 或测试库）
- [ ] Kafka 消费语义（offset、重投、幂等）的集成测试
- [ ] Canal 解析与重连的测试
- [ ] 多节点推送测试（依赖上面的跨节点方案）
- [x] 单元测试：红包算法、分页、错误码、缓存 key、连接管理、JWT、网关路由
- [x] 真实库结构回归测试（`TEST_MYSQL_DSN`）
- [x] docs/ 目录与 README 拆分
