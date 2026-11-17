# 文档目录

按**读者**与**用途**分四类，避免一篇文档同时承担多种职责。

| 目录 | 读者 | 内容 |
|---|---|---|
| [`requirements/`](requirements/) | 产品 / 需求方 | 做什么、验收标准；**不含实现细节** |
| [`design/`](design/) | 开发 | 架构、流程、取舍、为什么这样做 |
| [`guides/`](guides/) | 使用者 / 运维 | 怎么跑、怎么验证、故障怎么查 |
| [`ai/`](ai/) | AI 代理 / 协作者 | 约定、踩坑、不可违反的规则 |

## 索引

### requirements（需求）

| 文档 | 说明 |
|---|---|
| [offline-paketo](../project/offline-paketo/docs/architecture.md) | 纯 CNB 离线构建 |
| [offline-paketo-bk](../project/offline-paketo-bk/docs/requirements.md) | 离线 + 缓存 + 只输出应用层 |
| [offline-heroku-bk](../project/offline-heroku-bk/docs/design.md) | BK/Heroku 路线 |

> 需求文档目前放在各子项目 `docs/` 下，因为需求与项目强绑定；
> 本目录不重复维护，只做索引。

### design（技术）

| 文档 | 说明 |
|---|---|
| [base.md](design/base.md) | 配置化改造的设计取舍与早期踩坑 |
| [pipeline.md](design/pipeline.md) | 一条流水线的完整链路（配置 → 计划 → 执行） |
| [buildpack-split.md](design/buildpack-split.md) | buildpack 组件化方案与进度 |

### guides（对外）

| 文档 | 说明 |
|---|---|
| [verify.md](guides/verify.md) | L1–L5 验证手册（含 libc 一致性检查） |

### ai（AI 开发）

| 文档 | 说明 |
|---|---|
| [pitfalls.md](ai/pitfalls.md) | 18 条真实故障：现象 → 原因 → 修法 |

## 约定

1. **一个文档只属于一类**：需求不写实现，设计不写操作步骤，指南不写取舍。
2. **新增文档先选目录**，不要放到 `docs/` 根下。
3. 跨类引用用相对链接，不要复制内容。
- [协议分层](design/protocol-layers.md)：6 类协议的边界、解析器归属、删除清单与迁移顺序
