# 归档（历史）

这里放**已完成或已废弃**的设计记录：当时的取舍、踩过的坑、迁移过程。

- 只读，不再更新；命令与路径可能已失效
- 需要当前做法时，看 [`docs/design/`](../design) 或 [`docs/proposals/`](../proposals)

| 文档 | 内容 |
|---|---|
| [buildpack-split-progress.md](buildpack-split-progress.md) | buildpack 拆分进度与当时的迁移步骤（已完成） |
| [bk-build-logic.md](bk-build-logic.md) | `pack builder create` 时代的 BK 构建逻辑 |
| [bk-export-layout.md](bk-export-layout.md) | layout 导出实验记录（当前走 daemon） |
| [paketo-vs-heroku.md](paketo-vs-heroku.md) | Paketo 与 Heroku 上游对比（Heroku 已移除） |
| [offline-paketo-history.md](offline-paketo-history.md) | offline-paketo 的旧实现与结构变更 |
| [offline-paketo-bk-history.md](offline-paketo-bk-history.md) | offline-paketo-bk 的旧实现与导出方式演进 |

## 历史脚本

| 路径 | 说明 |
|---|---|
| [scripts/d1_union_package.py](scripts/d1_union_package.py) | D1 union 补层脚本（已被 v2 取代） |
| [scripts/dedup_smart.py](scripts/dedup_smart.py) | D1 多 tag + 硬链接去重（已被 v2 取代） |
| [scripts/d6_test_package.py](scripts/d6_test_package.py) | D6 多镜像 manifest 验证包（平台否决） |
| [scripts/d6_thin_package.py](scripts/d6_thin_package.py) | D6 薄层测试包（平台否决） |
