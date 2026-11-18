# 文档目录

按**读者**与**用途**分四类，避免一篇文档同时承担多种职责。

| 目录 | 读者 | 内容 |
|---|---|---|
| [`requirements/`](requirements/) | 产品 / 需求方 | 做什么、验收标准；**不含实现细节** |
| [`design/`](design/) | 开发 | 架构、流程、取舍、为什么这样做 |
| [`guides/`](guides/) | 使用者 / 运维 | 怎么跑、怎么验证、故障怎么查 |
| [`ai/`](ai/) | AI 代理 / 协作者 | 约定、踩坑、不可违反的规则 |
| [`proposals/`](proposals/) | 设计者 | **未来**：待拍板决策、演进路线、未实现方案 |
| [`archive/`](archive/) | 考古 | **历史**：已完成/废弃方案，只读 |

## 索引

### requirements（需求）

| 文档 | 说明 |
|---|---|
| [offline-paketo](../project/offline-paketo/docs/architecture.md) | 纯 CNB 离线构建 |
| [offline-paketo-bk](../project/offline-paketo-bk/docs/requirements.md) | 离线 + 缓存 + 只输出应用层 |

> 需求文档目前放在各子项目 `docs/` 下，因为需求与项目强绑定；
> 本目录不重复维护，只做索引。

### design（技术）

| 文档 | 说明 |
|---|---|
| [architecture.md](design/architecture.md) | **当前架构与两条案例流程**（含 mermaid 图） |
| [base.md](design/base.md) | 配置化改造的设计取舍与早期踩坑 |
| [pipeline.md](design/pipeline.md) | 一条流水线的完整链路（配置 → 计划 → 执行） |
| [buildpack-components.md](design/buildpack-components.md) | buildpack 组件契约与规则 |
| [protocol-layers.md](design/protocol-layers.md) | 协议分层、包 API、删除清单、**命名约束** |
| [build-chain.md](build-chain.md) | 从源码到 s-mart 的完整数据流 |
| [scaffold.md](design/scaffold.md) | 脚手架：从参考项目生成新项目 |

### proposals（未来）

| 文档 | 说明 |
|---|---|
| [base-roadmap.md](proposals/base-roadmap.md) | 基座待拍板决策与演进路线 |
| [bk-export-plan.md](proposals/bk-export-plan.md) | BK 导出改造的提交计划 |

### archive（历史）

| 文档 | 说明 |
|---|---|
| [buildpack-split-progress.md](archive/buildpack-split-progress.md) | buildpack 拆分进度（已完成） |
| [bk-build-logic.md](archive/bk-build-logic.md) | pack 时代的 BK 构建逻辑 |
| [bk-export-layout.md](archive/bk-export-layout.md) | layout 导出实验 |
| [paketo-vs-heroku.md](archive/paketo-vs-heroku.md) | Paketo 与 Heroku 对比 |

### guides（对外）

| 文档 | 说明 |
|---|---|
| [verify.md](guides/verify.md) | L1–L5 验证手册（含 libc 一致性检查） |
| [arm64.md](guides/arm64.md) | arm64 验证：架构边界、物料固定、断言步骤 |

### ai（AI 开发）

| 文档 | 说明 |
|---|---|
| [agents.md](ai/agents.md) | AI 入口：必读清单、命令、约束、自查 |
| [bksmartpacker-fork.md](ai/bksmartpacker-fork.md) | 与官方 bksmartpacker_v3 的差异、兼容性与升级方式 |
| [pitfalls.md](ai/pitfalls.md) | 18 条真实故障：现象 → 原因 → 修法 |
| [troubleshooting.md](guides/troubleshooting.md) | 部署后 Pod 不 Ready 的排查路径（进程/连接/goroutine 栈） |
| [protocol-spec.md](ai/protocol-spec.md) | spec/* 的 API 形状、未知字段、测试与检查清单 |
| [package-layout.md](ai/package-layout.md) | 包与目录约定 |
| [go-build.md](ai/go-build.md) | Go 构建、检查与自动化映射 |
| [git-workflow.md](ai/git-workflow.md) | 提交规范：一提交一事、禁止修补堆积、历史合并后验证 |

## 约定

1. **一个文档只属于一类**：需求不写实现，设计不写操作步骤，指南不写取舍。
2. **新增文档先选目录**，不要放到 `docs/` 根下。
3. 跨类引用用相对链接，不要复制内容。