# offline-paketo 架构说明

> 本文描述 `project/offline-paketo` 的**当前实现**。
> 仓库级流程见 [`docs/pipeline.md`](../../../docs/pipeline.md)，配置字段见 [`config/projects/offline-paketo.toml`](../../../config/projects/offline-paketo.toml)。

## 一、定位

在无外网环境构建 CNB builder 镜像：离线提供 lifecycle、Node/Python 运行时工具与
自定义 buildpack，让 `pack build` 不依赖任何外部下载。

与早期版本的区别：**流程已配置化**——物料、版本、镜像、顺序全部在配置里，
项目目录只保留 Dockerfile、buildpack 与入口脚本。

## 二、在仓库中的位置

```
Learn-cnb-paketo/
├── assets.lock.toml                     # 物料唯一事实源
├── config/projects/offline-paketo.toml  # 本项目的流水线定义
├── bundle/<platform>/binaries/<id>/<ver> # 共享物料（不入库）
├── script/                              # 执行器：cnb / fetch.sh / lib/
└── project/offline-paketo/              # 本项目
    ├── build.sh                         # 入口：cnb run <配置>
    ├── justfile                         # build / plan / fetch / verify / test
    ├── docker/                          # Dockerfile、buildpacks、order.toml
    └── examples/                        # 测试应用（不入库）
```

## 三、执行流程

```
build.sh
  └─ cnb run config/projects/offline-paketo.toml
       ├─ cnb profile validate   校验镜像/命令/流程引用
       ├─ cnb profile render     渲染执行计划（fetch_assets / image_build）
       └─ eval 计划
            ├─ fetch_assets  → script/fetch.sh（按 lock 下载/校验/解压）
            ├─ image_build   → script/lib/image-build.sh（podman/docker）
            └─ （无 command 阶段）
```

查看而不执行：

```bash
./build.sh --dry-run        # 打印计划与将执行的命令
just plan                   # 同上
just verify                 # 校验本项目物料（范围取自配置）
```

## 四、镜像

| 阶段 | Dockerfile | 产物 tag | 说明 |
|---|---|---|---|
| base | `docker/base.dockerfile` | `offline-builder-base:latest` | 系统依赖层 |
| run | `docker/run.dockerfile` | `offline-run:latest` | 运行时最小层 |
| builder | `docker/Dockerfile` | `offline-builder:latest` | lifecycle + buildpack + fnm/uv |

- 构建上下文是**仓库根**（`[docker] context = "."`），便于共享 `bundle/`
- Dockerfile 中物料路径形如 `bundle/${BUNDLE_ARCH}/binaries/lifecycle/${LIFECYCLE_VERSION}/…`
- `<ID>_VERSION` 由渲染器从配置注入，**版本只在 lock 里出现一次**

## 五、物料

由 `assets.lock.toml` 定义，`fetch.sh` 落到 `bundle/<platform>/binaries/<id>/<version>/`：

| 物料 | 版本 | 用途 |
|---|---|---|
| lifecycle | 0.21.13 | CNB 生命周期 |
| fnm | 1.39.0 | 安装 Node |
| uv | 0.11.26 | Python 环境 |

三态幂等：归档存在 / 哈希匹配 / 解压标记匹配，任一不满足就重做；
解压后再用 `unpack.expect` 校验产物是否齐全。

## 六、Buildpack

`docker/buildpacks/` 下三个自定义 buildpack + 一个官方 procfile（多阶段 COPY）：

| id | detect | build |
|---|---|---|
| `paketo-buildpacks/node` | `package.json` 等 | fnm 装 Node → npm/yarn/pnpm |
| `paketo-buildpacks/python` | `requirements.txt` 等 | uv 建 venv → pip 安装 |
| `paketo-buildpacks/procfile` | 官方镜像 | 官方实现 |

`docker/order.toml` 决定 detect 顺序：go → node → python → procfile。

## 七、已知缺陷（尚未修复）

这些属于 buildpack 实现层，与当前配置化改造无关：

1. **layer 元数据文件名错误**：node/python 写 `<layer>.launch`，规范要求 `<layer>.toml` +
   `[types]` 表 → 生命周期不认，运行时 `PATH` 不会生效
2. **launch 进程格式**：go 用 `command = ["…"]` 数组，规范要求字符串
3. **Python 重复进程类型**：`launch.toml` 写了 3 条 `type = "web"`
4. **并非真离线**：`fnm install` / `uv venv --python` 仍会联网下载 SDK

前三条会导致“构建成功但应用起不来”，修复计划见仓库 `docs/base.md` 的 P1。

## 八、历史变更

| 时间 | 变更 |
|---|---|
| 2026-11 | 删除 `scripts/*.just`（config/download/docker/info/log），流程改为配置驱动 |
| 2026-11 | `apps/` 改名 `examples/` 且不入库；物料统一到仓库根 `bundle/` |
| 2026-11 | 构建改用 named build context（`--build-context bundle=…`），Dockerfile 不再写仓库路径 |
| 2026-11 | 删除 `script/run.sh`，执行并入 `cnb run`；新增 `cnb dockerfile check` |
| 2026-11 | 物料目录改为 `<id>/<version>`，URL 支持 `{version}` 占位 |
