# BK 方案构建逻辑（归档）

> **归档**：`pack builder create` 时代的构建逻辑，已不再执行。
> 当前实现见 [`offline-paketo-bk/architecture.md`](../design/projects/offline-paketo-bk/architecture.md)。

# BK 方案构建逻辑

## 概览

BK 方案基于 **Cloud Native Buildpacks (CNB)** 标准流程，通过 `pack builder create` + `builder.toml` 构建 builder 镜像，配合 `bksmartpacker` 工具，将应用源码构建为蓝鲸 SmartPackage 格式的发布包（`.tar.gz`）。

核心流程：**pack builder create → bksmartpacker 编排 → CNB 容器内构建 → docker save 导出镜像 → 打包为 SmartPackage**。

三个环节全部对齐 BK 方案：

| 环节 | 方式 | 文件 |
|------|------|------|
| Builder 镜像构建 | `pack builder create` + `builder.toml` | `docker/builder.toml`, `docker/Dockerfile.build-image` |
| 应用构建编排 | `bksmartpacker` | `apps/build.sh` |
| 镜像导出 | 标准 lifecycle `exporter -daemon` | `docker/entrypoint.sh` `run_export()` |

---

## 架构图

```
                        Builder 镜像构建流程
┌──────────────────────────────────────────────────────────────────────┐
│  pack builder create --config docker/builder.toml \                  │
│    offline-builder-tools:latest  (build-image, 含工具)               │
│    offline-run-bk:latest       (run-image, 最小运行时)                │
│    + lifecycle tarball (file://)                                    │
│    + buildpack tar.gz   (file://)                                   │
│    ─────────────────────────────────────                             │
│    → offline-builder-bk:latest  (最终 builder)                      │
└──────────────────────────────────────────────────────────────────────┘

                        出包流程
┌──────────────────────────────────────────────────────────────────┐
│  apps/build.sh（入口脚本）                                        │
│  bksmartpacker-linux-amd64 \                                      │
│    --src  .../release-amd64 \                                    │
│    --dest .../bk_cmdb_saas-<ver>.tar.gz \                        │
│    --builder-image localhost/offline-builder-bk:latest \          │
│    --buildpack-cfg .../conf/buildpack.yaml                        │
└──────────────────────┬───────────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────────────┐
│  bksmartpacker 编排流程                                           │
│                                                                  │
│  1. 生成临时 workspace 目录                                        │
│  2. prepareFakeRunner() → 创建 FROM scratch 假运行镜像             │
│  3. 编译源码 → 生成 source.tgz（含 Procfile）                      │
│  4. docker run 启动 builder 容器，挂载：                           │
│     - source.tgz → /tmp/source.tgz                               │
│     - docker socket                                              │
│  5. 容器内执行 CNB 4 步骤（见下）                                   │
│  6. docker save 容器输出镜像 → workspace/<module>.tgz             │
│  7. 收集 app_desc.yaml / artifact.json / logo.png                │
│  8. tar.gz 打包 workspace/ → 最终 SmartPackage                    │
└──────────────────────┬───────────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────────────┐
│  Builder 容器内：CNB 4 步骤（BK 对齐，标准 lifecycle）             │
│  entrypoint.sh 全自动编排                                         │
│                                                                  │
│  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────────┐             │
│  │ Detect │→ │ Build  │→ │ Analyze│→ │  Exporter  │             │
│  │(1/4)   │  │(2/4)   │  │(3/4)   │  │(4/4)      │             │
│  └────────┘  └────────┘  └────────┘  └────────────┘             │
│                                                                  │
│  关键变化：第 4 步使用标准 lifecycle exporter -daemon               │
│  不再使用 docker import + docker build 自定义方案                  │
│  因为 bksmartpacker 创建 FROM scratch 的 fake runner              │
│  → exporter 输出的镜像只有 app 层，docker save 不包含 base 层       │
└──────────────────────┬───────────────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────────────┐
│  最终产物结构                                                      │
│  bk_cmdb_saas-<version>.tar.gz                                  │
│  ├── .Version            ← "cnb-image-layers"（bksmartpacker）    │
│  ├── app_desc.yaml       ← 应用描述                               │
│  ├── artifact.json       ← 构建元数据                             │
│  ├── logo.png            ← 应用图标                               │
│  └── web.tgz             ← docker save 导出的 app 镜像            │
│      └── <app-layer>.tar                                          │
│          └── app/bin/cmdb_webserver                               │
│          └── app/web/...                                          │
│          └── app/conf/...                                         │
│          └── app/Procfile                                         │
└──────────────────────────────────────────────────────────────────┘
```

---

## 各组件职责

### 1. `docker/builder.toml` — Builder 配置

> Paketo 标准格式，供 `pack builder create` 使用

```toml
[lifecycle]
version = "0.21.13"
uri = "file:///path/to/lifecycle-v0.21.13+linux.x86-64.tgz"

[[buildpacks]]
id = "paketo-buildpacks/go"
version = "0.1.0"
uri = "file:///path/to/go.tar.gz"

...

[stack]
id = "io.buildpacks.stacks.bionic"
build-image = "offline-builder-tools:latest"
run-image = "offline-run-bk:latest"
```

- 所有 URI 使用 `file://` 指向本地文件，**完全离线**
- build-image: 含全部构建工具（`offline-builder-tools:latest`），但无 lifecycle/buildpack
- `pack builder create` 自动将 lifecycle + buildpacks 叠加到 build-image 上

### 2. `Dockerfile.build-image` — Build Image

> 构建 `offline-builder-tools:latest`，供 builder.toml 的 build-image 引用

与旧版 `Dockerfile` 的区别：
- ❌ 不 COPY lifecycle（pack builder create 自动添加）
- ❌ 不 COPY buildpacks（pack builder create 自动添加）
- ❌ 不 COPY order.toml（pack 从 builder.toml 自动生成）
- ❌ 不含 OCI builder labels（pack 自动添加）
- ✅ 包含所有构建工具（jq, yq, fnm, uv, docker, pack）
- ✅ 包含 cnb 用户、platform 目录、entrypoint

### 3. `scripts/docker.just` — 构建编排

四层构建（BK 对齐）:

```
docker-build-base      → offline-builder-base:latest     # OS 层（debian）
docker-build-image     → offline-builder-tools:latest    # 工具层（jq/yq/fnm/uv/docker/pack）
docker-build-run       → offline-run-bk:latest           # 运行时层
pack-builder-create    → offline-builder-bk:latest       # 最终 builder（pack 叠加 lifecycle+buildpacks）
```

**旧的 Dockerfile 方式保留为备用**: `just docker-build-legacy`

### 4. `apps/build.sh` — 用户入口

```bash
# 自动架构感应
BKSMARTPACKER_BIN="${BKSMARTPACKER_BIN:-.../bksmartpacker-${ARCH}}"

"${BKSMARTPACKER_BIN}" \
  --src  "${SRC_DIR}" \
  --dest "${DEST}" \
  --builder-image "${BUILDER_IMAGE}" \
  --buildpack-cfg "${BUILDPACK_CFG}"  # ← BK 标准配置
```

- `--buildpack-cfg`: 指向 `bksmartpacker_v3/conf/buildpack.yaml`，定义各构建包的 oci-embedded 源
- 架构自动感应: 传入 `linux-arm64` 自动切换 `bksmartpacker-linux-arm64`

### 5. `bksmartpacker` — 构建编排器

> 源码位置：`cw-bksmartpacker_v3/pkg/builder/executors/docker.go`

核心逻辑 (`Execute()`)：

```
1. 创建临时 workspace 目录
2. prepareFakeRunner() → 创建 FROM scratch 的假运行镜像（用作 CNB_RUN_IMAGE）
3. 遍历 plan.Steps:
   a. buildStep() → 打包源码 → docker run builder 容器 → 容器内执行 CNB 流程
   b. exportImageLayer() → docker save 输出镜像 → workspace/<module>.tgz
4. buildSMartPackage() → 收集元数据文件 → tar.gz 打包
```

关键细节：
- **Builder 容器启动参数**（`buildStep` 中构造）：
  - `--mount type=bind,source=<docker.sock>,target=/var/run/docker.sock`
  - `--mount type=bind,source=<source.tgz>,target=/tmp/source.tgz`
  - `-e SOURCE_GET_URL=file:///tmp/source.tgz`
  - `-e OUTPUT_IMAGE=bk_cmdb_saas/web:<version>`
  - `-e CNB_RUN_IMAGE=fake-cnb-runner:latest`  ← fake runner（FROM scratch）
- `--buildpack-cfg` 定义构建包的下载源和版本

### 6. `entrypoint.sh` — 容器内 CNB 流水线

> 位置：`docker/entrypoint.sh`，镜像内路径：`/cnb/lifecycle/entrypoint.sh`

镜像内环境变量：
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LAYERS_DIR` | `/layers` | CNB 层目录 |
| `APP_DIR` | `/workspace` | 应用源码目录 |
| `PLATFORM_DIR` | `/platform` | 平台目录 |
| `RUN_IMAGE` | `offline-run-bk:latest` | 运行镜像（被 fake-runner 替代） |
| `OUTPUT_IMAGE` | — | 出口镜像名（bksmartpacker 传入） |
| `SOURCE_GET_URL` | — | 源码压缩包路径（bksmartpacker 传入） |

4 步骤详解：

#### Step 1: Detect
```
/cnb/lifecycle/detector -layers=/layers -app=/workspace -platform=/platform
```
- 根据 builder.toml 中的 `[[order]]` 检测应用类型
- 输出 `group.toml` 和 `plan.toml`

#### Step 2: Build
```
/cnb/lifecycle/builder -layers=/layers -app=/workspace -platform=/platform
```
- 执行匹配到的 buildpack（Go buildpack 编译二进制，Procfile buildpack 设置启动命令）

#### Step 3: Analyze
```
/cnb/lifecycle/analyzer -layers=/layers -run-image=<RUN_IMAGE> -daemon <OUTPUT_IMAGE>
```
- 分析前一构建的元数据，为增量构建做准备

#### Step 4: Export（标准 lifecycle exporter）
```
/cnb/lifecycle/exporter -daemon -layers=/layers -app=/workspace -run-image=<RUN_IMAGE> <OUTPUT_IMAGE>
```
- 使用标准 CNB exporter，替代旧版的 `docker import + docker build` 自定义方案
- bksmartpacker 创建 FROM scratch 的 fake runner 作为 run-image
- exporter 输出的镜像只包含 app 层，docker save 不包含 base 层
- exporter 自动处理：CNB launcher、进程类型、OCI labels、metadata

**与旧版的关键差异**：

| | 旧版 (docker import) | 新版 (lifecycle exporter) |
|---|---|---|
| 命令 | `docker import + docker build` | `/cnb/lifecycle/exporter -daemon` |
| 步骤数 | 5 步（含 detect_buildpack） | 4 步（标准 lifecycle） |
| CNB launcher | 手动 COPY | 自动嵌入 |
| 构建包检测 | 自定义 `detect_buildpack()` 函数 | 无需（by lifecycle） |
| 模板文件 | `export/Dockerfile.template` + `metadata.toml.template` | 已移除 |
| 镜像层数 | 仅 app 单层 + CNB 元数据 | app 层 + exporter 自动管理 |
| 体积控制 | 通过 fake runner 控制 | 同样通过 fake runner（BK 标准方案） |

---

## Builder 镜像构成

> 构建：`just docker-build`（定义在 `scripts/docker.just`）
> Build image：`Dockerfile.build-image`

镜像分层：

```
offline-builder-base:latest          # debian:trixie-slim + ca-certificates
                    ↓
offline-builder-tools:latest         # + jq, yq, fnm, uv, docker, pack, entrypoint
                    ↓  (pack builder create)
offline-builder-bk:latest            # + lifecycle(v0.21.13) + buildpacks(go/node/python/procfile)
```

镜像构成：
- **OS**: `debian:trixie-slim`
- **CNB Lifecycle**: v0.21.13（detector/builder/analyzer/exporter）
- **Buildpacks**:
  - `paketo-buildpacks/go` — 自定义 Go buildpack
  - `paketo-buildpacks/node` — 自定义 Node buildpack
  - `paketo-buildpacks/python` — 自定义 Python buildpack
  - `paketo-buildpacks/procfile` — 官方 Procfile buildpack（本地 tar.gz）
- **离线工具**: jq、yq、pack CLI（仅调试）
- **Runtime 工具**: fnm (Node)、uv (Python)、Docker CLI
- **入口**: `/cnb/lifecycle/entrypoint.sh`

### 架构参数

通过 `just` 的 `arch` 变量控制构建架构：

```bash
# 默认 amd64 构建
just download-all      # 下载 amd64 工具链
just docker-build      # 构建 amd64 builder

# arm64 构建
just download-all arch=linux-arm64    # 下载 arm64 工具链
just docker-build arch=linux-arm64    # 构建 arm64 builder
```

架构映射关系定义在 `project/*/pipeline.toml + assets.lock.toml`：

| 工具 | amd64 | arm64 |
|------|-------|-------|
| lifecycle | `linux.x86-64` | `linux.arm64` |
| docker | `x86_64` | `aarch64` |
| jq | `amd64` | `arm64` |
| yq | `amd64` | `arm64` |
| pack | `linux-amd64` | `linux-arm64` |
| fnm | `linux` | `arm64` |
| uv | `x86_64-unknown-linux-musl` | `aarch64-unknown-linux-musl` |

---

## 镜像构建

### 依赖

- `just` 任务运行器
- podman 或 docker
- 离线镜像源（`swr.cn-north-4.myhuaweicloud.com`）

### 构建命令

```bash
cd scripts

# 先下载所有工具链（离线模式）
just download-all

# 完整构建（BK 对齐，pack builder create）
just docker-build

# 查看配置
just info

# arm64 构建（替换 arch 参数）
just download-all arch=linux-arm64
just docker-build arch=linux-arm64

# 旧版 Dockerfile 方式（备用）
just docker-build-legacy
```

构建产物：`localhost/offline-builder-bk:latest`

### 分层构建（`docker.just`）

```
docker-build-base      → offline-builder-base:latest     # OS 层（debian）
docker-build-image     → offline-builder-tools:latest    # 工具层（jq/yq/fnm/uv/docker/pack）
docker-build-run       → offline-run-bk:latest           # 运行时层
pack-builder-create    → offline-builder-bk:latest       # 最终 builder（pack 叠加）
```

> 所有 stage 都支持 `arch=linux-arm64` 参数切换架构。

---

## 关键文件说明

### 新增文件

| 文件 | 用途 |
|------|------|
| `docker/builder.toml` | Paketo 标准 builder 配置（占位符模板） |
| `docker/Dockerfile.build-image` | Build image（供 pack builder create 引用） |

### 修改文件

| 文件 | 变更 |
|------|------|
| `docker/entrypoint.sh` | 移除 `run_export_image`/`run_export_layers`/`detect_buildpack`，改用标准 exporter |
| `scripts/docker.just` | 新增 `pack-builder-create`/`docker-build-image`/`docker-build-legacy` 配方 |
| `project/*/pipeline.toml + assets.lock.toml` | 新增 `build_image_tag` 变量 |
| `apps/build.sh` | 增加 `--buildpack-cfg`、架构自动感应、环境变量支持 |

### 删除文件

| 文件 | 原因 |
|------|------|
| `docker/export/Dockerfile.template` | 不再需要（改用 lifecycle exporter） |
| `docker/export/metadata.toml.template` | 不再需要（exporter 自动处理 metadata） |

---

## 数据流 & 文件流转

```
Host (bksmartpacker)                     Container (offline-builder-bk)
────────────────────                     ─────────────────────────────
release-amd64/                           
  main/bin/cmdb_webserver  ──→  BuildSourceTarball() → source.tgz
                                   │
                                   ├── mount ──────────────────→ /tmp/source.tgz
                                                               tar -xzf → /workspace/
                                                                         │
                                                                    lifecycle detect/build/analyze
                                                                         │
                                                                    exporter -daemon → daemon
                                                                         │
build-image (pack) ←──────────────────────────────────────────── pack builder create
                                                                         │
docker save ←─────────────────────────────────────────────────────── daemon
       │
       ▼
  workspace/web.tgz ──→ buildSMartPackage() ──→ bk_cmdb_saas-<version>.tar.gz
```

---

## 体积优化要点

| 优化措施 | 效果 | 实现位置 |
|----------|------|----------|
| lifecycle exporter + fake runner (FROM scratch) | web.tgz 仅含 app 层 | `entrypoint.sh` `run_export()`, bksmartpacker |
| 所有 buildpacks 使用 file:// URIs | 完全离线构建 | `docker/builder.toml`, `scripts/docker.just` |
| 构建包与 OS 解耦 | 语言无关，Node/Python 零适配 | `docker/builder.toml` `[[order]]` |
| Stdout 日志格式 | — | `entrypoint.sh` 各步骤 |

---

## 常见问题

### Q: 为什么新版使用标准 lifecycle exporter？

旧版使用 `docker import + docker build` 自定义方案，原因是标准 `exporter -daemon` 会输出包含 base image 层的完整镜像，导致 docker save 体积膨胀。但 BK 的 bksmartpacker 通过 `prepareFakeRunner()` 创建 FROM scratch 的 fake runner 作为运行镜像，exporter 基于 fake runner 输出的镜像只包含 app 层。因此可以安全使用标准 exporter，无需自定义方案。

### Q: 源码在容器内为什么可能没有 `main/` 前缀？

bksmartpacker 使用 `mholt/archiver` 库打包源码，路径为完整绝对路径。`tar -xzf` 解压后的目录结构取决于 tarball 中保存的路径格式。

### Q: 构建失败如何处理？

查看构建日志：
- bksmartpacker 日志：stdout/stderr 直接输出
- 容器内日志：被 bksmartpacker 捕获输出
- 错误排查方向：
  1. `Source file not found` → 检查 `--src` 路径
  2. Buildpack 匹配失败 → 检查 `builder.toml` 中的 `[[order]]` 和 buildpacks 版本
  3. `pack builder create` 失败 → 检查 lifecycle / buildpack tar.gz 路径
  4. exporter 失败 → 检查容器内 docker socket 挂载

### Q: pack builder create 和旧版 Dockerfile 有什么不同？

| | 旧版 Dockerfile | 新版 pack builder create |
|---|---|---|
| 构建命令 | `docker build -f Dockerfile` | `pack builder create --config builder.toml` |
| lifecycle | COPY bundle/binaries/... | file:// URI 自动提取 |
| buildpacks | COPY docker/buildpacks/... | file:// URI 指向 tar.gz |
| order.toml | COPY 到 /cnb/ | builder.toml 的 [[order]] 自动生成 |
| OCI labels | 手动 LABEL 指令 | 自动生成 |
| 规范合规 | 基本兼容 | Paketo 完全标准 |
