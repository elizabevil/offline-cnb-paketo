# offline-paketo 架构说明

> 本文描述 `project/offline-paketo` 的**当前实现**。
> 仓库级架构见 [`docs/design/architecture.md`](../../../docs/design/architecture.md)，
> 执行细节见 [`docs/design/pipeline.md`](../../../docs/design/pipeline.md)。

## 一、定位

在无外网环境构建 CNB builder 镜像：离线提供 lifecycle、Node/Python 运行时工具与
自研 buildpack，使 `lifecycle` 四阶段（detect → build → analyze → export）不依赖外部下载。

## 二、目录

```
project/offline-paketo/
├── pipeline.toml               # 流水线：base / builder 两条 stage
├── justfile                    # build / plan / fetch / verify / fixtures / test
├── docker/
│   ├── base.dockerfile         # 系统依赖层
│   ├── builder.dockerfile      # lifecycle + 自研 buildpack + fnm/uv
│   ├── lifecycle-entrypoint.sh # 容器入口（四阶段串联，可选）
│   ├── order.toml              # detect 顺序
│   └── apt-sources/            # 国内 apt 源
├── docs/
└── fixtures/                   # 测试应用（源码在 fixtures-src/）
```

buildpack 组件不在本项目内：它们是**仓库级资产**，位于 `buildpacks/paketo/`，
通过 `--build-context buildpacks=<repo>/buildpacks/paketo` 提供给 builder 构建。

## 三、执行流程

```mermaid
flowchart LR
    A["assets.toml / assets.lock.toml"] -->|"just fetch（cnb materials sync）"| B["bundle/&lt;platform&gt;/"]
    B --> C["pipeline.toml 的 builder stage"]
    C -->|"just build（cnb pipeline run）"| D["base stage：podman build base.dockerfile"]
    D --> E["builder stage：podman build builder.dockerfile"]
    E --> F["镜像 offline-builder:&lt;arch&gt;"]
```

常用命令：

```bash
just plan          # 打印最终 argv（不执行）
just build         # 执行两条 stage
just fetch         # 同步物料到 bundle/
just verify        # 校验物料落盘 + 配置结构
just test          # 对 fixtures 跑四次 lifecycle 构建
```

## 四、镜像

| stage | Dockerfile | 产物 tag | 内容 |
|---|---|---|---|
| base | `docker/base.dockerfile` | `offline-builder-base:<arch>` | 系统包（证书、curl、unzip）与 apt 源替换 |
| builder | `docker/builder.dockerfile` | `offline-builder:<arch>` | lifecycle、cnb CLI、fnm/uv、10 个 buildpack |

builder 的关键约定：

- `FROM ${BASE_IMAGE_TAG}`，tag 由 stage 的 `--build-arg` 传入
- `COPY --from=bundle` 取物料，`COPY --from=buildpacks` 取自研组件
- 非 root 用户 `cnb`(1000)，`/layers`、`/workspace`、`/platform` 可写

## 五、物料

由 `project/assets.toml` 声明、`project/assets.lock.toml` 固定哈希（路径以清单所在目录为根），`cnb materials sync` 落到
`bundle/<platform>/binaries/<id>/<version>/`。

本项目用到的物料（版本以 `assets.toml` 为准）：

| 物料 | 用途 |
|---|---|
| lifecycle | 四阶段可执行文件 |
| fnm | 安装 Node |
| uv | 安装 Python / 依赖 |
| cnb | 容器内执行器（可选） |
| procfile | Procfile 支持 |

三态幂等：归档存在 / 哈希匹配 / 解压标记匹配，任一不满足就重做；
解压后用 `unpack.expect` 校验产物齐全。

## 六、buildpack 与 order

组件实现见 [`docs/design/buildpack-components.md`](../../../docs/design/buildpack-components.md)。

| meta buildpack | 组成 |
|---|---|
| `paketo-buildpacks/go` | go-dist + go-build |
| `paketo-buildpacks/nodejs` | node-engine + npm-install + npm-start |
| `paketo-buildpacks/python` | cpython + pip-install |
| `paketo-buildpacks/procfile` | 官方预编译（物料） |

`docker/order.toml` 决定 detect 顺序：go → node → python → procfile。

## 七、已知限制

| 限制 | 说明 |
|---|---|
| 首次构建需要网络 | `node-engine` 用 `fnm install`、`cpython` 用 `uv python install`，缓存未命中时会联网；预置 `bundle/<platform>/sdk/` 或已有 `/layers/cache` 后离线 |
| procfile 依赖官方产物 | 来自物料 `procfile`，版本随 lock 更新 |
| 单架构镜像 | 镜像按 `<arch>` 打 tag，跨架构需分别构建 |
| 不产 run 镜像 | exporter 由使用方决定 run 镜像（本项目用 `debian:trixie-slim`） |

## 八、历史

早期实现与结构变更见 [`docs/archive/offline-paketo-history.md`](../../../docs/archive/offline-paketo-history.md)。
