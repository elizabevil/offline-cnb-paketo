# offline-paketo：历史变更

> **归档**：本项目早期实现与结构变更记录。
> 当前实现见 [`project/offline-paketo/docs/architecture.md`](../../project/offline-paketo/docs/architecture.md)。

## 已移除的实现

| 旧物 | 原因 | 现在 |
|---|---|---|
| `build.sh` | 与 `justfile` 重复 | `just --justfile project/offline-paketo/justfile build` |
| `script/lib/image-build.sh` | 镜像构建并入 stage | `pipeline.toml` 的 `base` / `builder` 两条 stage |
| `script/run.sh` | 执行并入统一入口 | `cnb pipeline run` |
| `docker/buildpacks/` | 组件升为仓库级资产 | `buildpacks/paketo/` |
| `examples/` | 与 `test/fixtures` 重复 | `fixtures/`（源码在 `fixtures-src/`） |
| `scripts/*.just`（config/download/docker/info/log） | 流程改为配置驱动 | `pipeline.toml` + `cnb materials` |

## 结构变更时间线

| 变更 | 说明 |
|---|---|
| 配置化 | 物料、版本、镜像、顺序从脚本移入 `pipeline.toml` |
| 命名构建上下文 | `--build-context bundle=…` / `buildpacks=…`，Dockerfile 不再写仓库路径 |
| 物料目录 | 改为 `<id>/<version>`，URL 支持 `{version}` 占位 |
| 配置位置 | `config/projects/<name>.toml` → `project/<name>/pipeline.toml`（项目自包含） |
| 脚手架 | 手写模板 → 从参考项目复制（`cnb init`） |

## 为什么不再维护旧路径

旧脚本与配置是两套事实来源：同一份版本号、同一套路径在两处维护，必然漂移。
现在唯一事实来源是 `pipeline.toml` + `assets.toml`/`assets.lock.toml`。
