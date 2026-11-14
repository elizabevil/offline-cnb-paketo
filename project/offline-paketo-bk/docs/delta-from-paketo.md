# 与 offline-paketo 的差异

两个项目是**递进关系**：`offline-paketo` 是纯 CNB 离线构建，本项目在其之上
增加 BK 平台契约。目录结构保持一致，差异如下。

| 维度 | offline-paketo | offline-paketo-bk |
|---|---|---|
| 目标平台 | CNB / pack | BK PaaS |
| 产物 | 完整镜像（含 OS） | **只含应用层**（run=scratch） |
| 入口契约 | `cnb run` | `cnb run` + `lifecycle-entrypoint.sh`（bksmartpacker 契约） |
| 交付形式 | 镜像 tag | s-mart.tgz（bksmartpacker 打包） |
| 测试夹具 | `test/fixtures`（普通应用） | `fixtures/`（含 `app_desc.yaml`，BK 结构） |
| 特有文档 | — | `docs/bk-build-logic.md` |

## 保持不变（刻意一致）

- `build.sh` / `justfile` 的位置与职责
- `docker/buildpacks/` 的 11 个组件（同一套自研实现）
- 离线优先级：挂载运行时 → `/layers/cache` → 镜像源
- 仓库级 `bundle/` 与 `config/projects/*.toml` 配置方式

## 命名差异（只做加法，不改名）

| 文件 | paketo | paketo-bk |
|---|---|---|
| Dockerfile | `base.dockerfile` / `builder.dockerfile` | `builder-base.dockerfile` / `builder-build.dockerfile` |

> 保持两项目原文件名，不做重命名或软链；新增能力一律用新文件表达。
