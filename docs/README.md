# 文档索引

| 文档 | 适合谁看 | 内容 |
|---|---|---|
| [architecture.md](architecture.md) | 想理解整体设计 | 服务拓扑、HTTP/消息链路、Kafka topic、Redis key、MySQL 表、一致性策略 |
| [services.md](services.md) | 要改某个服务 | 每个服务的职责、关键文件、对外接口、依赖 |
| [getting-started.md](getting-started.md) | 第一次跑起来 | 中间件准备、建库、启动顺序、联调验证、常见报错 |
| [error-handling.md](error-handling.md) | 写业务代码 | 错误码分段、Throw / WrapError 约定、HTTP 状态映射、异步场景注意点 |
| [redpacket.md](redpacket.md) | 看红包 | Redis 结构、Lua 抢包、状态机、过期退款、金额算法 |
| [testing.md](testing.md) | 加测试 | 测试分层、已覆盖内容、如何跑真实库回归 |
| [roadmap.md](roadmap.md) | 接手/排期 | 已知限制与待办，按优先级排列 |
| [fixes-2026-11-05.md](fixes-2026-11-05.md) | 想知道改了什么 | 本轮修复清单、每项的验证方式 |
| [code-review-2026-11-05.md](code-review-2026-11-05.md) | 复盘 | 全量代码审查报告（含未修项，与 roadmap 对应） |

## 阅读建议

1. 先读根目录 [README](../README.md) 了解项目定位
2. 再读 [architecture.md](architecture.md)，把"一条消息怎么走完全程"搞清楚
3. 想动手改代码前，务必读 [error-handling.md](error-handling.md)（这是本项目最容易踩的约定）
4. 想跑起来看 [getting-started.md](getting-started.md)
