# go-infinitechat 代码审查报告

- 审查时间：2026-11-05
- 代码基线：`main` @ `80c7c5b`（历史已清理，工作区干净）
- 规模：203 个 Go 文件 / 约 13,000 行 / 6 个服务
- 结论一句话：**架构分层和错误码设计是清晰的，但存在一批"写完没真正跑通"的代码——SQL 列名拼错、条件表达式写反、异步链路只接了半截。当前状态下，登录/注册经网关不可用、发红包必然 500、群消息推不出去、离线通知会丢。**

---

## 一、审查范围与方法

| 范围 | 方式 |
|---|---|
| `common/`（错误码、中间件、Kafka、etcd、dto、utils） | 逐行人工审查 |
| `service/gateway/`（代理、路由、鉴权、CORS） | 逐行人工审查 |
| `service/user/api`、`service/user/rpc`、`service/user/model` | 人工 + 交叉核对 |
| `service/realtime`（WebSocket、消费者） | 人工 + 交叉核对 |
| `service/offline`（Canal、消费者、离线读取） | 人工 + 交叉核对 |
| `service/redpacket`（Lua、发送/领取、过期扫描） | 人工 + 交叉核对 |

关键结论都对照了依赖库源码验证，避免把注释当事实：
- `go-queue@v1.2.2/kq/config.go`：`ForceCommit bool json:",default=true"`
- `go-queue@v1.2.2/kq/queue.go:235-240`：只有 `ForceCommit=false` 才跳过提交

---

## 二、总体评价

**做得好的地方**
- 分层清楚：gateway / api / rpc / model / consumer 职责边界明确，没有把业务塞进 handler。
- 错误码体系（4xxxx / 5xxxx / 9xxxx）设计合理，`SetErrorHandlerCtx` 统一了响应格式，"业务错误 200 + 业务码，系统错误 500 + 脱敏" 这条约定坚持得不错。
- `txctx` 用 context 传递事务 session 的思路正确，能自动复用外层事务。
- 注释密度高，大部分关键设计（Redis 热点边界、红包 Lua、一致性哈希）都有解释。
- 红包金额拆分算法经核对是正确的（每份 ≥1 分、总和精确、无死循环）。

**问题集中在三类**
1. **SQL 手写后没跑过**：3 处列名/引号错误，对应功能 100% 失败（发红包、拉黑、好友申请列表）。
2. **条件表达式写反**：`err == nil` 写成 `err != nil`、判空用了错误变量、布尔组合少一个 `!`，共 5 处，都是静默返回空/默认值。
3. **异步链路没有闭环**：Kafka 消费者压根没启动、消费失败仍提交 offset、Redis 改了但事件没发出。

---

## 三、P0：功能直接不可用 / 安全漏洞

### 3.1 安全（4 条）

#### P0-1 越权：身份完全由客户端决定，网关设置的 `X-User-Id` 无人读取

- 位置：`service/gateway/internal/middleware/auth.go:75`；`service/user/api/internal/logic/**/*.go`（30+ 处）
- 证据：
  ```go
  // gateway：校验 token 后把身份塞进 header
  r.Header.Set("X-User-Id", userIdStr)
  ```
  ```go
  // 但下游全部改用路径/body 里的 userId
  return l.svcCtx.FriendService.DeleteFriend(l.ctx, req.UserId, req.ReceiveUserId), nil
  ```
  全仓库 grep：没有任何一处读取 `X-User-Id`。
- 影响：任意登录用户可以操作他人——删好友、拉黑、改密码/头像、查余额明细、踢人、退群、通过别人的好友申请。这是本审查里最严重的问题。
- 修法：加一个中间件，把 `X-User-Id` 解析成 int64 放进 ctx；logic 层统一用 ctx 里的身份覆盖 `req.UserId`，并与路径参数比对，不一致直接 403。

#### P0-2 手机号验证码硬编码 `123456`

- 位置：`service/user/api/internal/logic/user/loginlogic.go:72`、`registerlogic.go:41`
- 证据：`code := "123456" // 写死123456`
- 影响：知道手机号即可登录/注册任意账号。邮箱分支走了 Redis 校验，手机分支是开发桩。
- 修法：与邮箱分支一致，走 `LoginCodePhonePrefix+account` 的 Redis 校验，校验通过即删 key。

#### P0-3 offline 服务无鉴权，只凭 sessionId 就能读任意会话记录

- 位置：`service/offline/api/offline.go`（只挂了 `RecoverMiddleWare`）、`logic/offlinedata/historicalmessagelogic.go:37`、`offlinemessagelogic.go:38`
- 证据：
  ```go
  sessionId := req.SessionId   // 请求体里根本没有 userId
  ```
- 影响：遍历自增/可猜的 sessionId 即可拉取他人全部聊天历史（热数据 + MySQL 冷数据）。
- 修法：请求体或网关 header 带 userId，服务端校验 `user_session` 里该用户确实属于这个会话。

#### P0-4 红包接口无身份校验，且不校验"不能领自己的包"

- 位置：`service/redpacket/api/internal/logic/receiveredpacketlogic.go:37`、`types/types.go:52`
- 证据：`ExecuteReceiveLua(l.ctx, req.RedPacketId, req.UserId)`，`UserId` 直接来自 JSON body；Lua 只做 `HEXISTS` 防重复领。
- 影响：可冒用任意 userId 领包/发包；发送者能领回自己的包。
- 修法：身份取鉴权上下文；领取前比对 `rp.SenderId`。

### 3.2 网关配置与路由漂移（1 条）

#### P0-5 网关白名单路径全部过期，登录/注册被 401 拦截

- 位置：`service/gateway/etc/gateway.yaml`（WhiteList） vs `service/user/api/internal/handler/routes.go:132/152/163/181`
- 证据：
  ```yaml
  WhiteList:
      - "/api/user/sendCaptcha"
      - "/api/user/register"        # 实际是 POST /api/user/users
      - "/api/user/loginPassword"   # 实际是 POST /api/user/login
      - "/api/user/loginCode"       # 实际是 POST /api/user/login
  ```
- 影响：经网关（10010）无法登录、无法注册，只有发验证码能过。直连 8104 才能工作。
- 修法：白名单改为 `/api/user/login`、`/api/user/users`；并把白名单比较改成"前缀 + 后缀匹配"或直接在 user.api 里标注免鉴权，避免再次漂移。

### 3.3 SQL 错误导致整个功能 100% 失败（4 条）

#### P0-6 `updateed_time` → 扣余额必失败 → 发红包 100% 500

- 位置：`service/user/model/user_balance/userbalancemodel.go:70`
- 证据：
  ```go
  "UPDATE %s SET `balance` = `balance` - ?, `updateed_time` = NOW() WHERE `user_id` = ? AND `balance` >= ?"
  ```
  表结构 `user_balance.sql:5` 是 `updated_time`（且自带 `ON UPDATE CURRENT_TIMESTAMP`）。
- 影响：`DeductBalanceTx` 每次报 1054 Unknown column → 事务回滚 → `sendredpacketlogic.go:58` 抛错 → 发红包完全不可用。退款路径 `AddBalanceTx` 不写该列，反而正常。
- 修法：删掉这个赋值（列会自更新）或改成 `updated_time`。

#### P0-7 `update_time` → 拉黑/取消拉黑必失败

- 位置：`service/user/model/friend/friendmodel.go:107`
- 证据：`"update %s set `status` = ?, `update_time` = now() where ..."`；`friend.sql:7` 是 `updated_time`。
- 影响：`BlockFriend` / `UnblockFriend`（`friend_service.go:411/431`）稳定 500。
- 修法：改为 `updated_time` 或直接省略。

#### P0-8 好友申请总数 SQL 把占位符包进了反引号

- 位置：`service/user/model/apply_friend/applyfriendmodel.go:76`、`:90`
- 证据：
  ```go
  "select count(*) from %s where `sender_id` = ? or `receiver_id = ?`"   // 整个谓词成了字段名
  ...
  if err != nil { return nil, 0, nil }                                  // 分页错误被吞
  ```
- 影响：好友申请列表（`GET /api/contact/users/{userId}/apply`）稳定 500；即使修掉 count，分页失败也会伪装成"空列表"。
- 修法：`` or `receiver_id` = ? ``；`:90` 改为 `return nil, 0, err`。

#### P0-9 群成员查询条件写成了 `user_id`，应为 `session_id`

- 位置：`service/user/model/user_session/usersessionmodel.go:85`
- 证据：
  ```go
  query := fmt.Sprintf("select `user_id` from %s where `user_id` = ? and `status` = 0", m.table)
  // 调用方传的是 sessionId
  ```
- 影响：`GetGroupMemberIds` 基本恒为空 → `realtime/consumer/message_consumer.go:97` 拿不到群员 → **群消息只推给发送者自己**。极端情况下会把群消息推给 `user_id` 恰好等于 `session_id` 的无关用户（越权泄露）。
- 修法：`where `session_id` = ? and `status` = 0`（现有 `idx_session_status` 正好覆盖）。

### 3.4 条件写反 / 死分支（4 条）

#### P0-10 批量取用户信息：成功的结果被 `continue` 掉，接口恒返回空

- 位置：`service/user/rpc/internal/logic/batchgetuserinfoslogic.go:35`
- 证据：
  ```go
  u, err := l.svcCtx.UserModel.FindOne(l.ctx, item)
  if err == nil || u == nil { continue }   // 成功也跳过
  ```
- 影响：离线消息列表、红包详情拿不到发送者昵称/头像（`offlinemessagelogic.go:112`、`canal_client.go:280`、`redpacket_service.go:159`）。同时是逐条 `FindOne` 的 N+1。
- 修法：`if err != nil || u == nil { continue }`，并改批量 `where user_id in (...)`。

#### P0-11 好友申请"已有申请则更新"分支是死代码 → 重复申请撞唯一键 500

- 位置：`service/user/api/internal/service/apply_friend_service.go:70`
- 证据：
  ```go
  existApply, err := s.ApplyFriendModel.FindOneBySenderIdReceiverId(...)
  if err == nil && existFriend != nil {   // 应为 existApply
  ```
  即使 `existFriend` 非 nil，第 66 行已经抛过"已经是好友关系"，走不到这里。
- 影响：重复点击"加好友"总是 INSERT，撞 `UNIQUE(sender_id, receiver_id)` → 500。
- 修法：条件改 `existApply != nil`，命中待处理/已读则更新 message 并重置未读后 return。

#### P0-12 领取红包：已领 / 已过期分支没有 `return`，落到兜底抛 500

- 位置：`service/redpacket/api/internal/logic/receiveredpacketlogic.go:43-55`、`:75`
- 证据：
  ```go
  if resultCode == constants.AlreadyReceived {  // -1
      resp.Amount = ...
  }                                              // 没有 return
  if resultCode == constants.AlreadyExpired {    // -2
      resp.Status = ...
  }                                              // 没有 return
  ...
  if resultCode <= 0 { common.ThrowWithMsg(common.SystemError, "Lua返回未知结果码") }
  ```
- 影响：用户连点两次"领取"→ 500「系统内部异常」；领过期红包同样 500。Lua 三个业务码里两个是死分支。
- 修法：两个分支末尾 `return resp, nil`。

#### P0-13 好友详情的好友状态恒为 -1

- 位置：`service/user/api/internal/service/friend_service.go:235`
- 证据：`if err != nil && friendRelation != nil { vo.Status = ... } else { vo.Status = -1 }`
- 影响：好友详情页永远显示"非好友"状态。
- 修法：`if err == nil && friendRelation != nil`。

### 3.5 实时链路（3 条）

#### P0-14 Kafka 消费者从未启动，在线推送与离线通知全链路是死的

- 位置：`service/realtime/realtime.go:49-51`
- 证据：
  ```go
  if len(c.Kafka.Brokers) > 0 {
      // go
  }
  ```
  `startMessageConsumer` / `startNotificationConsumer` 全仓库没有调用点（函数体完整写好却没人调）。
- 影响：`message-topic` 没人消费 → 所有在线消息不推送；`system-notification-topic` 没人消费 → 系统通知不推送。
- 修法：`go startMessageConsumer(c, svcCtx)` 与 `go startNotificationConsumer(c, svcCtx)`。

#### P0-15 离线通知的 Pusher 指向了自己正在消费的 topic

- 位置：`service/realtime/internal/svc/servicecontext.go:65`
- 证据：
  ```go
  notificationPusher = kq.NewPusher(c.Kafka.Brokers, constants.KafkaSystemNotificationTopic)
  // 注释写的是 store-notification-topic
  ```
  而 offline 服务消费的是 `KafkaStoreNotificationTopic`。
- 影响：用户离线时通知被推回自己消费的 topic → 自消费循环，永远不落库；同时真实落库 topic 无人投递。
- 修法：改成 `constants.KafkaStoreNotificationTopic`。

#### P0-16 消息消费者在 nil 判断前解引用指针，一条畸形消息崩掉进程

- 位置：`service/realtime/internal/consumer/message_consumer.go:73`、`:75`、`:79`
- 证据：
  ```go
  common.Infof("... toUserId=%d ...", msgReq.SenderId, *msgReq.ReceiverId, ...)  // 73
  if msgReq.ReceiverId != nil { ... }                                            // 79 才判空
  ```
- 影响：`kq` 的消费循环用 `RoutineGroup.Run`（无 recover），任何生产者发一条 `sessionType=0` 且无 `receiverId` 的消息（例如红包消息）就会 panic 崩进程。
- 修法：把 nil 判断提到最前；或在 consumer 入口加 `defer recover()` 兜底。

### 3.6 数据一致性（3 条）

#### P0-17 红包主表用非事务 `Insert`，事务回滚后留下孤儿红包

- 位置：`service/redpacket/api/internal/logic/sendredpacketlogic.go:62`
- 证据：
  ```go
  _, insertErr := l.svcCtx.RedPacketModel.Insert(ctx, &red_packet.RedPacket{...})
  ```
  go-zero `CachedConn` 直接走 `cc.db`，不读 `txctx` 里的 session，因此这条 INSERT 不在第 56 行开启的事务里。
- 影响：余额日志写失败回滚扣款后，仍留一条 `status=0`、无 Redis 池、不在 ZSet 的孤儿红包——永不退款。
- 修法：改用 `InsertTx`（`redpacketmodel.go:37` 已有），三步同事务。

#### P0-18 过期退款"先查状态再无条件 UPDATE"，并发/重投会重复退款

- 位置：`service/redpacket/api/internal/consumer/redpacket_consumer.go:186`、`:189`、`:208-210`
- 证据：
  ```go
  if rp.Status == constants.StatusExpired { return nil }
  if rp.Status != constants.StatusNotCompleted { return nil }
  ...
  rp.Status = constants.StatusExpired
  err = c.rpSvc.RedPacketModel.UpdateTx(ctx, rp)   // SQL 是 where red_packet_id = ?，无状态条件
  ```
- 影响：Kafka 至少一次投递 + 3 并发消费，同一红包两个消费者都读到 `status=0`、算出同一 `remain`，各退一次 → 发送者被退两次，平台凭空多钱。
- 修法：条件更新 `UPDATE red_packet SET status=2 WHERE red_packet_id=? AND status=0`，`RowsAffected==1` 才退款；或 `SELECT ... FOR UPDATE`。

#### P0-19 offline 落库失败仍提交 offset（注释与框架默认值相反）

- 位置：`service/offline/api/internal/consumer/message_store_consumer.go:39-42` + `offline.go` 的 `KqConf`（未设 `ForceCommit`）
- 证据：
  ```go
  // 返回 error → go-queue 不提交 offset ...
  return err
  ```
  但 `go-queue@v1.2.2/kq/config.go`：`ForceCommit bool json:",default=true"`；`queue.go:235-240` 只在 `ForceCommit=false` 时 `continue`。
- 影响：MySQL 故障期间消息照常 commit，重启也不重投，**消息永久丢失**。
- 修法：两个 offline 消费者的 `KqConf` 都显式 `ForceCommit: false`，并在 Consume 内做有限退避重试 + 死信。

---

## 四、P1：数据不一致 / 并发 / 性能

| # | 位置 | 问题 | 修法 |
|---|---|---|---|
| 1 | `apply_friend_service.go:201`、`:208` | 通过好友申请非原子：先 `UpdateTx` 把申请置为已通过（无外层事务，自动提交），再 `AddFriend` 另起事务；后者失败则申请已"通过"但无好友关系，重试报"该申请不可被处理" | 三步合进同一个 `txctx.WithTransaction`，Kafka 通知移到提交后 |
| 2 | `receiveredpacketlogic.go:85-93` | LPOP 已拿走金额后 Kafka 推送失败无补偿：钱既不入账也不回池，Hash 25h 后消失 | 失败时 Lua 补偿 `HDEL`+`LPUSH` 回池；根治用事务性发件箱 |
| 3 | `package redpacket_service.go:206` | `CalculateRemainAmount` 把 Redis 错误吞成 `0`，消费者照样置"已过期"并摘 ZSet → 退款归零且不可恢复 | 返回 `(int64, error)`，出错让 kq 重试 |
| 4 | `sendredpacketlogic.go:94` | 事务提交、扣款完成后才初始化 Redis 池与 ZSet，任一步失败无补偿 → 已扣款但不可领、扫不到、永不退 | Redis 初始化失败补偿退款；补"status=0 且不在 ZSet"的对账任务 |
| 5 | `schedular/expiration_dispatcher.go:101-107` | 扫描脚本先 `ZREM` 再让 Go 投 Kafka，崩溃/投递失败即永久漏退款；且锁 TTL 1s < 单轮预算 5s，互斥失效 | 消费成功后再摘；TTL ≥ 预算 + 余量 |
| 6 | `servicecontext.go:121-138`（realtime） | etcd `KeepAlive` 用的 ctx 带 30s 超时且 `defer cancel()`，函数返回即取消 → 注册约 30s 后失效，节点从服务发现消失（僵尸节点） | 用不取消的 ctx，并把 KeepAlive 通道异常作为重新注册信号 |
| 7 | `notification_consumer.go:63`、`:69` | 推送的是裸 `messageId` 而非消息 JSON，客户端 `JSON.parse` 必失败；在线判定用 `Hexists(wsServerUri)`，而登录时就已写入该字段 → 判成"在线"直接 return，既不推送也不落库 → 通知静默丢失 | 推送 `val`；在线判定改为查真实连接或给路由 key 加 TTL |
| 8 | `ws_server.go:232-247` | store/message 双写不原子、共用一个 5s ctx；`kq.NewPusher` 默认异步批量，`Push` 返回 nil ≠ 已写入 Kafka → 客户端收到的成功是假的 | 关键路径用同步推送或本地重试队列；两写加补偿 |
| 9 | `ws_server.go:417` | 推送只查本节点连接表，而 `message-topic` 用全节点共享消费组 → 一条消息只被一个节点消费，接收者在别的节点就丢（`wsServerUri` 路由信息完全没用上） | 按 sessionId hash 分区消费，或消费后转发到目标节点 |
| 10 | `ws_server.go:119`、`:391` | 路由无 TTL，进程被 kill 后残留永不过期；`Hget`+`Hdel` 非原子，跨节点会误删新节点记录 | 路由 key 加 TTL + Lua 原子摘除 |
| 11 | `validatesinglemessagelogic.go:99`、`:120` | 好友状态缓存 key 无分隔符（(1,23) 与 (12,3) 撞 key），且 `Set` 不传过期时间 = 永久；DB 错误也被缓存成 -1，一次抖动让这对用户永久"非好友" | key 加 `:`，`Set` 带 `friendCacheTTL`；DB 错误不落缓存 |
| 12 | `validatesinglemessagelogic.go:99` vs `friend_service.go:362` | 读写 key 格式不一致（RPC 不带 `:`，API 清理带 `:`）→ 清理从未生效，与 #11 叠加成永久脏缓存 | 抽公共 `FriendStatusCacheKey(uid, fid)` 两边共用 |
| 13 | `friendmodel.go:63` | 事务版删除走裸 `session.ExecCtx`，不触发 go-zero 缓存失效 → 删好友后旧关系仍在（默认 7 天），重新加好友被拒"已经是好友关系" | 事务提交后显式 `Del` 受影响的 cache key |
| 14 | `friend_service.go:294` | 好友列表 N+1：先全量拉关系（无 limit），循环内每人 2 次查询，分页在内存做 | 批量查 + SQL 分页 |
| 15 | `balance_service.go:40`、`:60` | 条件写反：`err != nil && errors.Is(err, sqlc.ErrNotFound)` → 只把"没找到"当异常抛，真正的数据库故障被忽略，随后报"余额记录不存在" | 改为 `err != nil && !errors.Is(...)` |
| 16 | `recover.go:25` | 日志打的是 `ErrorCode.Error()`，只输出 Message，`Cause`（真实 SQL/Redis 错误）永远不进日志 | 打 `bizErr.Cause`，或给 `ErrorCode` 实现 `Format` |
| 17 | `apply_friend_service.go:112` + `friend_service.go:316` | 好友列表/申请列表循环里逐条 `GetUserById`、逐条查会话 | 批量 `in` 查询 |
| 18 | `cannal_client.go:83`、`:96-99` | 用 `connector.Get`（canal-go 内部在拉取时即 `Ack`）而非 `GetWithOutAck`，位点先于处理前进；235/237/271/274 全用 `_ =` 吞 Redis 错误 → Redis 抖动即静默丢离线数据 | 改 `GetWithOutAck`，整批成功后再 Ack，`handleMessage` 返回 error |
| 19 | `cannal_client.go:63-88` | 连接/订阅失败直接 `return`，goroutine 静默死亡；断线不重连但 `Connected` 仍为 true → 对着死连接空转，离线功能停摆而服务"健康" | 加重连 + 健康检查 |
| 20 | `historicalmessagelogic.go:42-47`、`messagemodel.go:36` | 游标只有秒级 `datetime`，同秒多条消息被永久跳过（无 messageId 兜底） | 游标改成 `(created_time, message_id)` 复合 |
| 21 | `historicalmessagelogic.go:63` | 热数据补差额用 `hotBoundary` 而非游标 `requestTime`，依赖"Redis 必留满 7 天"，Redis 不完整就出现永久空洞 | 用 `min(requestTime, hotBoundary)` 作为边界 |
| 22 | `getunreadnotificationlogic.go:36` | `pageNum=0` → `offset=-20` → `LIMIT -20,20` SQL 报错 500（`default=1` 只在参数缺失时生效） | 方法内兜底 `pageNum<1 → 1` |
| 23 | `markasreadlogic.go:55` + `systemnotificationmodel.go:52` | `ExecNoCacheCtx` 不清缓存，而 `Insert` 是带缓存的 `ExecCtx`、TTL 7 天 → 第二次标记已读返回 false，读路径 7 天内仍认为未读 | 更新后 `DelCache` 或统一设 TTL |
| 24 | `gateway/middleware/auth.go:67`、`user_service.go:75` | Token/refreshToken 明文写进日志 | 只记前 6 位或 hash |
| 25 | `user_service.go:83-87` | 密码用 MD5 + 固定盐 `"USER"` | 改 bcrypt / argon2id |
| 26 | `constants/jwt_constant.go:12`、`user-api.yaml` | JWT 密钥硬编码在源码；MySQL root 密码、MinIO 密钥明文在受跟踪配置文件里 | 移到环境变量/本地未跟踪配置 |

---

## 五、P2：工程性与可维护性

1. **零测试**：203 个文件、0 个 `*_test.go`。上面 19 个 P0 里有 8 个只要有一个"改完跑一遍"的集成测试就能拦住。
2. **两套日志并存**：`common.Logger`（zap，Debug 级，控制台）与 `logx`（配置里 `Level: error`）。同一次排障要在两处找日志。
3. **gateway 细节**：`Resolver.Next()` 用写锁做纯读操作（每请求串行化）；每个请求打一条 `Infof`；`ListenAndServe` 出错只打日志不退出；没有 `ReadHeaderTimeout`/优雅关闭。
4. **snowflake 固定 `WorkerID=1`**：多服务/多实例生成同一时间片的 ID 存在碰撞可能；`NewNode` 失败时 `node` 为 nil，`NextInt` 会空指针 panic。
5. **`expiration_dispatcher` 的 `LRANGE 0 -1`**：大拼手气包会阻塞 Redis（你 CLAUDE.md 里已记为待办）。
6. **`InitRedisPool` 对尚不存在的 records key 做 `EXPIRE`**：不生效 → records Hash 永不过期，隐性内存泄漏。
7. **发红包无幂等**：`clientMessageId`/`PreventDuplicatePrefix` 都定义了却没用，重复提交会重复扣款。
8. **`offline` 的 `unreadOfflineCount` 无上限**：可传 2^31-1，整 ZSet 拉进内存再 `LIMIT 20 亿`。
9. **`AddBalance` / `AddBalanceTx` 未校验 `RowsAffected`**：用户余额行不存在时静默"成功"，钱凭空消失。
10. **`recover.go` 尾部残留注释** `// ============ 1. Redis ============`，说明文件被裁剪过，建议清理。
11. **`balance_service.InitializaBalance`** 拼写错误（Initialize）；`common.ThrowIfWithMsg(userId <= 0, common.UserAlreadyExists, ...)` 用错了错误码。
12. **`.api` 里 `range=[1:100]` 在部分 logic 未生效**（logic 层用 `int32` 强转），超大 pageSize 会全表扫描，超过 2^31 会溢出成负数。

---

## 六、建议的修复顺序

**第一批：让核心链路能跑（半天）**
1. P0-6/7/8/9 四处 SQL（改 4 行）
2. P0-10/11/12/13 四处条件（改 4 行）
3. P0-5 网关白名单
4. P0-14/15 实时链路两个启动/topic 错

**第二批：止血安全（1 天）**
5. P0-1 身份透传（一个中间件 + logic 覆盖，30 处，机械但量最大）
6. P0-2 验证码
7. P0-3 offline 会话归属校验
8. P0-4 红包身份 + 不能领自己的包

**第三批：钱和消息不能错（2~3 天）**
9. P0-17/18/19 事务与幂等
10. P1-1/2/3/4 红包链路闭环
11. P1-6 etcd 续租、P1-7 通知在线判定

**第四批：体验与工程化**
12. 缓存 key 统一 + TTL（P1-11/12/13）
13. 补一层最关键的集成测试：登录 → 发消息 → 落库 → 推送；发红包 → 领取 → 退款

---

## 七、审查盲区（未覆盖）

- `*_gen.go`、`*.pb.go`、goctl 生成的 handler：按生成代码对待，未逐行审查。
- 前端/客户端、部署脚本、CI：仓库中不存在。
- 运行时数据（Redis 里实际 key、MySQL 实际数据）：本次是静态审查 + 依赖库源码核对，未连库验证。
- 标注"待确认"的两处：`friend`/`red_packet` 缓存 TTL 依赖 go-zero 默认值（7 天）；`user_session.status` 是否将来改软删（影响 P1-9 的严重度）。
