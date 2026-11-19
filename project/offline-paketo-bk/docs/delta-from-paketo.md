# 与 offline-paketo 的差异

两个项目是**递进关系**：`offline-paketo` 是纯 CNB 离线构建，本项目在其之上
增加 BK 平台契约。目录结构保持一致，差异如下。

| 维度 | offline-paketo | offline-paketo-bk |
|---|---|---|
| 目标平台 | CNB / pack | BK PaaS |
| 产物 | 完整镜像（含 OS） | **只含应用层**（run=scratch） |
| 入口契约 | `cnb pipeline run` | `cnb pipeline run` + `lifecycle-entrypoint.sh`（BK 契约） |
| 交付形式 | 镜像 tag | s-mart.tar.gz（bksmartpacker 打包） |
| 测试夹具 | `test/fixtures`（普通应用） | `fixtures/`（含 `app_desc.yaml`，BK 结构） |
| 特有文档 | — | `docs/architecture.md`（历史与 BK 逻辑见 `docs/archive/`） |

## 保持不变（刻意一致）

- `pipeline.toml` / `justfile` 的位置与职责
- 仓库级 `buildpacks/paketo/` 的组件（同一套自研实现）
- 离线优先级：挂载运行时 → `/layers/cache` → 镜像源
- 仓库级 `bundle/` 与 `project/*/pipeline.toml` 配置方式

## 命名差异（只做加法，不改名）

| 文件 | paketo | paketo-bk |
|---|---|---|
| Dockerfile | `base.dockerfile` / `builder.dockerfile` | `builder-base.dockerfile` / `builder-build.dockerfile` |

> 保持两项目原文件名，不做重命名或软链；新增能力一律用新文件表达。

## 层包导出：默认走 layout

podman 下 lifecycle 的 `-daemon` 有单连接限制，因此 BK 默认走 layout：

```
容器内：exporter -layout → /layers/oci
        cnb 打包镜像目录 → /cnb-layer/<module>.tgz（挂载自宿主）
宿主：  bksmartpacker 复制 <module>.tgz → workspace → s-mart.tar.gz
```

`BKS_EXPORT_MODE=daemon` 才会退回 `image save`（仅真 dockerd）。
cnb 不读 `BKS_*` 变量；`BKS_*` 是 bksmartpacker 自己的开关。
