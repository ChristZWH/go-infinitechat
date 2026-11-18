# 红包模块代码审查（2026-11-18）

本文件记录红包模块当前存在的问题，按严重度分级。修复后请在对应条目上标注「已修复」及提交哈希。

> 更新（2026-11-18 第二批）：已修复 #3、#4、#5、#6、#7、#9、#10；剩余 #8（会话成员校验）待做。

## P0 资金安全 / 严重

### 1. 发红包幂等缺失（已修复）

- **位置**：`service/redpacket/api/internal/logic/sendredpacketlogic.go`
- **问题**：`clientMessageId` 只被写进消息体，从未用于去重。客户端超时重试或双击时，会重复扣款、重复发红包。
- **修复**：`clientMessageId` 非空时，事务前用 Redis `SETNX prevent:duplicate:{senderId}:{clientMessageId}` 抢占（TTL 24h）。
  - 抢占失败 → 读缓存结果直接返回（真正幂等）；
  - 处理中（pending）→ 返回「请稍后重试」；
  - 失败（panic/error）→ `defer` 删除幂等键，允许重试，避免把失败误判为已提交。

### 2. 红包份数无上限（已修复）

- **位置**：`sendredpacketlogic.go` 的 `validateSendRequest`、`types/constants/redpacket_constant.go`
- **问题**：只校验了 `TotalCount > 0`，没有上限。大份数（如 10 万份）会让 `InitRedisPoolLua` 收到 10 万个 ARGV 并循环 `RPUSH` 10 万次，造成 Redis 长时间阻塞和超大 EVAL 包（DoS 向量）。
- **修复**：新增 `MaxTotalCount = 1000`，`TotalCount > MaxTotalCount` 直接返回参数错误。

## P1 可靠性 / 一致性

### 3. 查询接口把 DB 错误当成「红包不存在」

- **位置**：`getredpacketbasicinfologic.go:37`、`getredpacketdetaillogic.go:36`
- **问题**：`ThrowIfWithMsg(findErr != nil, common.NotFoundError, ...)` 把任何查询错误都当成「红包不存在」。DB 故障时会返回业务错误（HTTP 200）而不是 500，误导客户端与监控。
- **修复**：区分 `sqlx.ErrNotFound` 与真实 DB 错误，后者返回 `SystemError`（参考 `receiveredpacketlogic.go:40-44` 已修好的写法）。

### 4. 查询接口无权限校验（信息泄露）

- **位置**：`getredpacketbasicinfologic.go`、`getredpacketdetaillogic.go`
- **问题**：不校验请求者是否是该红包的发送者 / 领取者 / 会话成员。任意登录用户拿到 `redPacketId` 即可查询红包详情，包括**领取者名单与各自金额**。
- **修复**：查询前校验请求者与红包的关联（发送者、已领取记录、或所属会话成员）。

### 5. 分页参数未 clamp，负 offset 会报错

- **位置**：`types.go:29-30`、`red_packet_receive` 模型的 `FindPageByRedPacketId`
- **问题**：`pageNum` / `pageSize` 只有 `default=1/10`（仅字段缺省时生效）。客户端显式传 `pageNum=0` 或负数时，`offset := (pageNum-1)*pageSize` 变负，SQL `LIMIT -10, 10` 报错；`pageSize` 也无上限。
- **修复**：查询前 clamp（`pageNum >= 1`、`pageSize` 限制在 `[1, 100]`）。

### 6. `CompletedConsumer` 无条件 Update 全字段

- **位置**：`redpacket_consumer.go:87-90`
- **问题**：`rp.Status = StatusCompleted; Model.Update(ctx, rp)` 是整行更新，没有 `WHERE status = 0` 条件。若红包已被 `ExpirationConsumer` 标记为 `Expired`（如 completed 事件延迟消费），会被覆盖回 `Completed`。
- **修复**：改为条件更新（`UPDATE status = 1 WHERE red_packet_id = ? AND status = 0`），与 `MarkExpiredIfNotCompleted` 风格一致。

### 7. 消费者对 `ErrNotFound` 返回 err，导致永久重试

- **位置**：`redpacket_consumer.go:82-86`（Completed）、`120-123`（Expiration）
- **问题**：`FindOne` 返回 `ErrNotFound` 时 `return err`，而队列 `ForceCommit=false`，这条消息会永久重试（毒消息）。
- **修复**：`ErrNotFound` 是永久条件，应 `return nil` 跳过；只有其它错误才返回 err 触发重试。

## P2 健壮性 / 小问题

### 8. 领取 / 发送不校验会话成员资格

- **位置**：`receiveredpacketlogic.go:40-44`、`sendredpacketlogic.go`
- **问题**：只校验「不能领自己的红包」，没有校验请求者是否属于红包所在会话。知道 `redPacketId` 的非会话成员也能抢 / 发。
- **修复**：领取 / 发送前通过 `user.rpc` 校验会话成员资格。

### 9. `RedPacketSendRequest.Type` 是死字段

- **位置**：`types.go:69`
- **问题**：`Type int32` 定义后从未使用。
- **修复**：删除该字段，或在 Handler 层忽略。

### 10. `AlreadyReceived` 分支的 `Status` 是占位值

- **位置**：`receiveredpacketlogic.go:57`
- **问题**：重复领取时 `resp.Status = constants.StatusNotCompleted` 只是占位，客户端需要靠 `Message` 判断。
- **修复**：回查红包真实状态（低频路径，可接受多一次查询），或明确在响应协议中约定该字段在此场景无意义。

## 修复优先级建议

| 优先级 | 条目 |
|---|---|
| 立刻 | 1（已修）、2（已修） |
| 本周 | 3、5、7 |
| 按需 | 4、6、8、9、10 |
