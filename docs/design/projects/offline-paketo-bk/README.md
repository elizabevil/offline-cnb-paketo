# offline-paketo-bk 设计索引

> 本目录放 `project/offline-paketo-bk` 的**需求、架构与决策**。
> 使用说明在 `project/offline-paketo-bk/docs/`。

## 文档与职责

| 文档 | 职责 |
|---|---|
| [requirements.md](requirements.md) | 需求与非目标 |
| [architecture.md](architecture.md) | 当前实现架构 |
| [build-chain.md](build-chain.md) | 构建链路：谁负责什么、产物落盘 |
| [delta-from-paketo.md](delta-from-paketo.md) | 与 offline-paketo 的差异 |
| [smart-app-v2-analysis.md](smart-app-v2-analysis.md) | 官方 smart-app-builder v2 分析 |
| [v2-adoption-decisions.md](v2-adoption-decisions.md) | v2 吸收 / 抛弃决策 |
| [v2-migration-scope.md](v2-migration-scope.md) | v2 改造范围与实现结果 |

## 阅读顺序

```text
requirements → architecture → build-chain → delta-from-paketo
                                             └→ smart-app-v2-analysis
                                                  └→ v2-adoption-decisions
                                                       └→ v2-migration-scope
```
