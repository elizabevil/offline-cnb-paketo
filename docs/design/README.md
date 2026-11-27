# design/ 索引

`docs/design/` 只放**当前实现**的技术设计：架构、职责边界、协议、物料规则、项目决策。
历史方案在 `docs/archive/`，踩坑在 `docs/guides/`，待决策在 `docs/proposals/`。

## 一、基座（跨项目）

| 文档 | 职责 |
|---|---|
| [base/architecture.md](base/architecture.md) | 整体分层与两条案例 |
| [base/pipeline.md](base/pipeline.md) | 流水线如何被执行 |
| [base/constraints.md](base/constraints.md) | 基座约定：路径、布局、校验规则 |
| [base/scaffold.md](base/scaffold.md) | 从参考项目生成新项目 |
| [base/protocol-layers.md](base/protocol-layers.md) | 协议分层总览 |
| [base/protocol/](base/protocol/) | spec / lifecycle / 子项目契约 |
| [base/materials/](base/materials/) | 物料声明、锁、同步 |

## 二、运行时

| 文档 | 职责 |
|---|---|
| [runtime/README.md](runtime/README.md) | 运行时文档索引 |
| [runtime/concepts.md](runtime/concepts.md) | 概念、角色、拓扑、导出模式、singleton |
| [runtime/rootless-recommendation.md](runtime/rootless-recommendation.md) | 非 root 方案选择与命令 |
| [runtime/cache.md](runtime/cache.md) | runtime image 缓存需求与决策 |

测试矩阵属于**测试参考**，见 [guides/runtime-tests/](../guides/runtime-tests/)。

## 三、组件

| 文档 | 职责 |
|---|---|
| [components/buildpack-components.md](components/buildpack-components.md) | 自研 buildpack 组件契约 |
| [components/bksmartpacker/README.md](components/bksmartpacker/README.md) | bksmartpacker fork / 对比 |

## 四、项目

| 文档 | 职责 |
|---|---|
| [projects/offline-paketo/README.md](projects/offline-paketo/README.md) | 纯 CNB 离线构建设计 |
| [projects/offline-paketo-bk/README.md](projects/offline-paketo-bk/README.md) | BK 侧需求、架构与 v2 决策 |

## 规则

- 一个文档只承担一种职责；交叉引用用链接，不复制内容。
- 跨项目内容放 `base/`，运行时放 `runtime/`，项目专属放 `projects/<name>/`。
- 测试结果/矩阵放 `docs/guides/`，不进 design。
