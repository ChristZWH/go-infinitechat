# 测试

## 1. 运行

```bash
go test ./...            # 全部单元测试（无外部依赖，秒级）
go test -race ./...      # 竞态检测
go vet ./...             # 静态检查
```

真实库结构回归测试默认跳过，需要 MySQL：

```powershell
$env:TEST_MYSQL_DSN = "root:password@tcp(127.0.0.1:3306)/infiniteChat?charset=utf8mb4&parseTime=True&loc=Local"
go test ./service/user/model/ -v
```

```bash
# macOS / Linux
TEST_MYSQL_DSN='root:password@tcp(127.0.0.1:3306)/infiniteChat?charset=utf8mb4&parseTime=True&loc=Local' \
  go test ./service/user/model/ -v
```

## 2. 分层

| 层次 | 依赖 | 覆盖内容 |
|---|---|---|
| 纯函数单测 | 无 | 红包金额拆分、分页计算、错误码、缓存 key、金额转换、JWT |
| 逻辑单测（fake model） | 无 | RPC logic 的控制流（批量查用户、群成员） |
| 中间件/网关单测 | 无（httptest） | 白名单、401 响应、路由前缀匹配 |
| 并发单测 | 真实 WebSocket（httptest） | 连接管理器双向映射、踢旧连接、`-race` |
| 结构回归（可选） | MySQL | 手写 SQL 的列名/语法是否与真实表一致 |

## 3. 已有测试文件

| 文件 | 关键用例 |
|---|---|
| `service/redpacket/api/internal/algorithm/redpacket_algorithm_test.go` | 总额精确、每份 ≥1 分、余数分配、切点法退化不死循环（每用例跑 50 轮） |
| `common/model/dto/page_dto_test.go` | pageNum/pageSize 兜底、Pages/HasNext/HasPrevious、pageSize 截断 |
| `common/common/error_code_test.go` | 4xxxx 与 5xxxx 分段、`errors.As` 还原、Cause 保留 |
| `common/model/constants/constants_test.go` | 好友缓存 key 的分隔符与方向性（(1,23) ≠ (12,3)） |
| `service/realtime/internal/channelmgr/channel_manager_test.go` | 双向映射、踢旧连接后反向表清理、RemoveByConn 幂等、并发读写 |
| `service/user/rpc/internal/logic/behavior_test.go` | BatchGetUserInfos 不再恒空、群成员透传与错误上抛 |
| `service/user/api/internal/utils/jwt_util_test.go` | sub/iat/exp、签名真的生效、错误密钥必须失败 |
| `service/redpacket/api/internal/service/redpacket_service_test.go` | 元↔分 无损、两位小数限制、分转元格式化 |
| `service/gateway/internal/handler/route_test.go` | 前缀匹配、首个命中、无路由 |
| `service/gateway/internal/middleware/auth_test.go` | 白名单精确匹配、缺 token 401、白名单放行 |
| `service/user/model/schema_test.go` | 用 `EXPLAIN` 校验 6 条手写 SQL 的列名（需 `TEST_MYSQL_DSN`） |

## 4. 这些测试在防什么

| 曾经的真实 bug | 对应测试 |
|---|---|
| 批量取用户信息恒返回空（`err == nil` 写成跳过） | `behavior_test.go` |
| 好友缓存 key 缺分隔符，导致按别人的好友关系放行 | `constants_test.go` |
| 拼手气在 `remainAmount < count` 时死循环 | `redpacket_algorithm_test.go` |
| 网关白名单与路由漂移，登录被 401 | `auth_test.go` |
| 连接被新连接顶掉后，旧连接断开误删新连接 | `channel_manager_test.go` |
| 金额用 float 转换导致 ±1 分 | `redpacket_service_test.go` |
| SQL 列名写错（`updateed_time` / `update_time` / 反引号错位 / `user_id` 误用） | `schema_test.go` |
| JWT `iat` 带纳秒导致校验方报 "used before issued" | `jwt_util_test.go` |

## 5. 写新测试的约定

- 优先纯函数 / fake 接口，不要为了测试引入数据库依赖；确需真实库时用 `TEST_MYSQL_DSN` 环境变量跳过式设计。
- fake model 的写法：内嵌接口 + 只覆盖用到的方法。

```go
type fakeUserModel struct {
    user.UserModel            // 内嵌 nil 接口，兜住其余方法
    users map[int64]*user.User
}
func (f *fakeUserModel) FindOne(_ context.Context, id int64) (*user.User, error) { ... }
```

- 涉及并发的用例一定要在 `-race` 下跑通。
- 表驱动 + `t.Run(名字)`，用例名用中文描述业务语义。

## 6. 尚未覆盖

- HTTP handler → logic → 真实 MySQL 的端到端链路（需要测试库或 sqlmock）
- Kafka 消费的 offset 语义（需要嵌入式 Kafka 或打桩 kq）
- Canal 解析与重连
- 多节点推送（本身尚未实现）
