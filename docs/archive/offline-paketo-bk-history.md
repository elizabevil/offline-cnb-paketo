# offline-paketo-bk：历史变更

> **归档**：本项目从 pack 时代到当前实现的变更记录。
> 当前实现见 [`project/offline-paketo-bk/docs/architecture.md`](../../project/offline-paketo-bk/docs/architecture.md)；
> pack 时代的详细流程见 [`bk-build-logic.md`](bk-build-logic.md)。

## 已移除的实现

| 旧物 | 原因 | 现在 |
|---|---|---|
| `pack builder create` + `builder.toml` | 改为直接构建镜像 | `pipeline.toml` 的 `base` / `builder` stage |
| `scripts/` 与 `just download-all` | 统一到 cnb | `cnb materials sync` / `just fetch` |
| `docker/buildpacks/` | 组件升为仓库级资产 | `buildpacks/paketo/` |
| `apps/` 与 `apps/build.sh` | 应用构建由 bksmartpacker 编排 | `just smart` / `just pack-case` |
| 自建 `entrypoint.sh` 四阶段脚本 | 编排移到 Go | `cnb lifecycle run`（入口脚本只转交） |
| run 镜像（`offline-run-bk`） | 导出只含应用层 | `CNB_RUN_IMAGE=scratch` |

## 导出方式演进

| 阶段 | 方式 | 问题 |
|---|---|---|
| 初版 | `docker import` + `docker build` 自定义 | 不是标准 lifecycle |
| 过渡 | exporter `-daemon` | podman 单连接限制（`singleton connection already in use`） |
| 当前默认 | exporter `-layout` + `CNB_LAYER_ARCHIVE` | 无 daemon 依赖；bksmartpacker 直接取层包 |
| 可选 | `BKS_EXPORT_MODE=daemon` | 仅真 dockerd 适用 |

## 结构变更时间线

| 变更 | 说明 |
|---|---|
| 配置化 | 流程从 `scripts/` 移入 `pipeline.toml` |
| 组件上移 | buildpack 从项目内移到仓库级 `buildpacks/` |
| 契约收口 | BK 环境变量由 `spec/platform` 解析，入口脚本只转交 |
| 配置位置 | `config/projects/<name>.toml` → `project/<name>/pipeline.toml` |
