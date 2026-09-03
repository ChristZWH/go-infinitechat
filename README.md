# go-infinitechat

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
