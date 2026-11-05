# 身份与鉴权

## 1. 三层结构

```
客户端
  │  Authorization: Bearer <access_token>
  ▼
gateway  ── ① 查 Redis access_token:<token> 得到 userId
           ② 校验路径里 /users/{id} 的 id == userId（不一致直接 403）
           ③ 删除客户端自带的 X-User-Id，写入可信值
  │  X-User-Id: <userId>
  ▼
服务入口  ── ④ IdentityMiddleware 解析 X-User-Id → context
  │
  ▼
logic    ── ⑤ identity.UserId(ctx) 取身份，覆盖请求参数里的 userId
```

关键点：**身份只有一个来源**——网关注入的 `X-User-Id`。URL 路径与请求体里的 userId 只是"定位参数"，不再作为身份凭据。

## 2. 路径语义约定

`/users/{id}` 中**第一个** id 永远是"当前操作者"：

| 路径 | 操作者 | 说明 |
|---|---|---|
| `/api/user/users/123` | 123 | 查自己资料 |
| `/api/user/users/123/balance` | 123 | 查自己余额 |
| `/api/contact/users/7/friend/8` | 7 | 7 与 8 的关系；8 是对方，不参与身份校验 |
| `/api/group/users/9` | 9 | 9 加入的群 |

网关用 `/users/(\d+)` 取**第一次**出现的 id 做比较，因此 `createUserId` / `receiveUserId` / `friendId` 这类参数不会被误判。

没有 `/users/{id}` 段的接口（如 `/api/group/{sessionId}/members`、`/api/offline/...`）不做路径校验——这类接口的归属是"会话成员"，必须由服务端自行判断（见第 6 节）。

## 3. 各层代码

### 网关：写入与校验

`service/gateway/internal/middleware/auth.go`

```go
if isPathUserIdMismatch(path, userIdStr) {
    m.responseForbidden(w)   // 403 {"code":40300,"message":"无权访问他人的数据"}
    return
}
r.Header.Del(identity.HeaderUserId)      // 防伪造：先删客户端自带的
r.Header.Set(identity.HeaderUserId, userIdStr)
```

### 服务：解析进 context

`service/user/api/internal/middleware/identity.go`

```go
if raw := r.Header.Get(identity.HeaderUserId); raw != "" {
    if userId, err := strconv.ParseInt(raw, 10, 64); err == nil && userId > 0 {
        r = r.WithContext(identity.WithUserId(r.Context(), userId))
    }
}
```

只解析、不拒绝：白名单接口（登录/注册）本来就没有身份，中间件不能因为缺头就阻断。

### logic：以 ctx 身份为准

```go
if userId, ok := identity.UserId(l.ctx); ok {
    req.UserId = userId      // 覆盖路径/请求体里的值
}
```

没有身份时（直连服务调试）保留原参数，行为与改造前一致。

## 4. 当前覆盖范围

| 位置 | 状态 |
|---|---|
| 网关路径校验（覆盖所有 `/users/{id}` 接口） | ✅ 已上线 |
| 网关注入 + 删除伪造头 | ✅ 已上线 |
| `common/identity`（ctx 读写） | ✅ 已有测试 |
| user.api `IdentityMiddleware` | ✅ 已注册到 `server.Use` |
| `UserInfo` / `GetUserBalance` / `GetBalanceDetail` / `DeleteFriend` | ✅ 已改为 ctx 身份 |
| 其余 user.api 接口（好友申请、群操作、拉黑等） | ⏳ 由网关路径校验兜底，logic 尚未逐个迁移 |
| offline 接口（历史/离线消息） | ⏳ 仍只按 sessionId 取数，需要会话归属校验 |
| redpacket 领取 | ⏳ userId 仍来自请求体 |

## 5. 测试

| 用例 | 文件 |
|---|---|
| 路径 id 与 token 一致/不一致/缺段/非数字 | `service/gateway/internal/middleware/auth_test.go` |
| 403 响应体 | 同上 |
| 中间件解析合法/非法头、永不阻断 | `service/user/api/internal/middleware/identity_test.go` |
| ctx 读写、非法 ID 不写入、子 context 继承 | `common/identity/identity_test.go` |

## 6. 还没做的部分

1. **logic 全量迁移**：剩余 20+ 接口仍用 `req.UserId`。虽然网关已经拦住跨用户访问，但"服务被直连"时没有第二道防线。建议按目录批量替换，并用一个 fake ctx 的单测模式守护。
2. **会话维度授权**：`/api/group/{sessionId}/members`、`/api/offline/...` 需要校验"我在这个会话里"。可以在 middleware 里读 `pathvar` 后查 `user_session`，也可以在 logic 里统一调 `UserSessionService`。
3. **红包身份**：领取/发送的 userId 应取 ctx，同时校验 `不能领自己的包`。
4. **RPC 侧**：`user.rpc` 的调用方目前信任传入的 senderId（realtime/offline 传的），后续可加内部鉴权（mTLS 或签名）。
