# Offline Paketo — 项目需求详情、架构与整体逻辑

---

## 一、项目定位

**项目名称**：`offline-paketo` — 离线 Paketo CNB 构建系统

**核心问题**：Paketo 官方构建包在标准模式下需要实时从 GitHub 下载 lifecycle、buildpack、SDK 等组件。但在**离线/内网/断网环境**下，这些下载会全部失败，导致 `pack build` 无法运行。

**解决思路**：预先将所有依赖（lifecycle 二进制、buildpack 层、语言 SDK）下载到本地目录，通过 Dockerfile COPY 进镜像，让 `pack` / `lifecycle` 完全离线执行。

---

## 二、需求详情

### 2.1 功能需求

| # | 需求 | 说明 |
|---|------|------|
| F1 | 预离线下载 Lifecycle / fnm / uv | 获取 `lifecycle` 二进制（detector、analyzer、builder、launcher、restorer），版本可配置 |
| F2 | 运行时离线安装 Node SDK | 通过 `fnm` 工具动态安装 Node.js，版本可配置 |
| F3 | 运行时离线安装 Python SDK | 通过 `uv` 工具动态安装 Python，版本可配置 |
| F5 | 多架构支持 | 同时支持 `linux-amd64` 和 `linux-arm64` |
| F7 | 使用 curl 与代理支持 | 企业内网必须走 HTTP 代理，所有下载显式使用 `--proxy` |
| F8 | 缓存分离 | 构建缓存（Dockerfile COPY）与运行时缓存（SDK 持久化）分离 |
| F9 | 单命令下载所有 | `just download-all` 一键完成 |
| F10 | Docker 镜像构建 | `just docker-build` 一键构建完整 builder 镜像 |
| F11 | 宿主机 APT 镜像源注入 | 预复制宿主镜像源到构建上下文，apt-get 走国内源加速 |

### 2.2 非功能需求

| # | 需求 | 说明 |
|---|------|------|
| N1 | 零外部依赖 | 宿主仅需 `just`、`curl`、`tar`、`unzip`、`sha256sum` |
| N2 | 幂等性 | 重复执行不破坏已有文件 |
| N3 | 可审计 | 所有下载来源、版本、路径在 `config.just` 中集中管理 |
| N4 | 最小基镜像 | 使用 `debian:trixie-slim` 作为最终构建环境 |
| N5 | 良好日志 | 每个步骤有清晰的 emoji 前缀、代理状态、成功/失败标识 |
| N6 | 容器运行时自适应 | 自动检测 podman / docker |

---

## 三、整体架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         just 编排层 (scripts/)                           │
│  justfile  (汇总入口)                                                    │
│  ├── config.just   (版本/URL/代理/架构映射)                              │
│  ├── download.just (下载 lifecycle/fnm/uv)                               │
│  ├── docker.just   (Docker 构建)                                         │
│  ├── info.just     (信息查看/依赖检查/文件列表/清理)                      │
│  └── log.just      (日志辅助)                                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  构建缓存 (bundle/)                          Docker 构建上下文            │
│  ┌──────────────────────────────────┐       ┌─────────────────────┐     │
│  │  archives/{arch}/                │       │  docker/             │     │
│  │  ├── lifecycle-*.tgz             │       │  ├── Dockerfile      │     │
│  │  ├── fnm-*.zip                   │       │  ├── base.dockerfile │     │
│  │  └── uv-*.tar.gz                 │       │  ├── apt-sources/    │     │
│  │                                  │       │  │   └── sources.list│     │
│  │  binaries/{arch}/                │       │  ├── order.toml      │     │
│  │  ├── lifecycle/ (detector,...)   │       │  └── buildpacks/     │     │
│  │  ├── fnm                         │       │      ├── node/       │     │
│  │  ├── uv                          │       │      ├── python/     │     │
│  │  └── uvx                         │       │      └── procfile/   │     │
│  └──────────────────────────────────┘       └─────────────────────┘     │
│                                                                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  最终产物: 离线 Builder Docker 镜像                                      │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  /cnb/lifecycle/      — lifecycle 二进制                          │   │
│  │  /usr/local/bin/fnm   — Node 版本管理器                           │   │
│  │  /usr/local/bin/uv    — Python 包管理器                           │   │
│  │  /usr/local/bin/uvx   — uv 工具执行器                             │   │
│  │  /cnb/buildpacks/                                                  │   │
│  │  ├── paketo-buildpacks_node/0.1.0/     (自定义)                   │   │
│  │  ├── paketo-buildpacks_python/0.1.0/   (自定义)                   │   │
│  │  └── paketo-buildpacks_procfile/5.13.4 (官方 Paketo 多阶段提取)    │   │
│  │  /cnb/order.toml      — detect 顺序                                │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  运行时: pack build / lifecycle CLI                                     │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │ 步骤 1: detect   → 遍历 buildpacks, 匹配项目类型                   │   │
│  │ 步骤 2: build    → fnm/uv 安装 SDK + npm/pip 安装依赖              │   │
│  │ 步骤 3: export   → 打包为 OCI 镜像                                 │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 三.1 架构要点

| 维度 | 说明 |
|------|------|
| **单一镜像** | 一个 Builder 镜像内含全部 3 种 Buildpack，运行时自动 detect 项目类型 |
| **构建缓存** | `bundle/` 是 Docker build 阶段 COPY 进去的，包含 lifecycle + fnm/uv 工具链 |
| **基础层分离** | base.dockerfile 仅装系统依赖，Dockerfile 叠加上层构建产物，可单独缓存 base 层 |
| **APT 源注入** | `docker/apt-sources/sources.list` 预置宿主镜像源，构建时 apt-get 走国内源 |
| **架构参数化** | `BUNDLE_ARCH` build arg 驱动 COPY 路径，`just arch=linux-arm64 docker-build` 切换架构 |
| **代理统一** | 所有 curl 下载使用 `http_proxy= https_proxy= curl ... --proxy` 双重保障 |
| **容器运行时自适应** | 自动检测 podman/docker，无需手动指定 |

---

## 四、目录结构

```
offline-paketo/
├── scripts/
│   ├── justfile            # 汇总入口：仅 import + _default
│   ├── config.just         # 配置：版本、URL、代理、架构映射
│   ├── download.just       # 下载：lifecycle / fnm / uv / all
│   ├── docker.just         # Docker 构建：docker-build-base / docker-build
│   ├── info.just           # 工具：info / check-deps / list / clean
│   └── log.just            # 日志辅助（被 info / download import）
│
├── docker/
│   ├── Dockerfile          # 最终 Builder 镜像：lifecycle + tools + buildpacks
│   ├── base.dockerfile     # 基础 OS 层：apt 依赖
│   ├── order.toml          # CNB detect 顺序
│   ├── apt-sources/
│   │   └── sources.list    # 宿主机 APT 镜像源（预复制，构建时生效）
│   └── buildpacks/
│       ├── node/
│       │   ├── buildpack.toml
│       │   └── bin/{detect,build}
│       ├── python/
│       │   ├── buildpack.toml
│       │   └── bin/{detect,build}
│       └── procfile/       # （本地副本, 实际构建使用官方 Paketo 镜像多阶段提取）
│           ├── buildpack.toml
│           └── bin/{detect,build}
│
├── bundle/
│   ├── archives/{arch}/    # 原始压缩包（可重新解压/校验）
│   └── binaries/{arch}/    # 解压后的可执行文件
│       ├── lifecycle/      # detector / analyzer / builder / launcher / restorer
│       ├── fnm             # Node 版本管理器
│       ├── uv              # Python 包管理器
│       └── uvx             # uv 工具执行器
│
├── docs/
│   └── architecture.md     # 本文档
│
├── apps/                   # （预留）测试用示例应用
├── cache/                  # （预留）运行时 SDK 缓存目录
└── .env                    # PROXY_URL 等环境变量
```

---

## 五、just 编排层设计

### 5.1 模块拆分

```
scripts/
├── justfile          仅 import 5 个模块 + _default (just --list)
├── config.just       纯变量：版本号、URL、架构映射、路径
├── download.just     下载 lifecycle / fnm / uv
├── docker.just       Docker 构建相关
├── info.just         信息/检查/列表/清理
└── log.just          日志显示辅助函数
```

justfile 不包含任何功能代码，仅做汇总，确保单一入口点。

### 5.2 Recipe 总览

```
Recipe                     depends on         说明
────────────────────────────────────────────────────────────────────
just --list                —                  列出所有 recipe（默认）
just info                  —                  显示当前配置
just check-deps            —                  检查宿主机依赖工具
just download-all          download-*         一键下载全部
just download-lifecycle    —                  下载 lifecycle 二进制
just download-fnm          —                  下载 fnm
just download-uv           —                  下载 uv
just list                  —                  列出已下载文件
just clean                 —                  清理下载文件
just docker-build-base     —                  构建 base OS 镜像
just docker-build          docker-build-base  构建完整 Builder 镜像
```

### 5.3 变量覆盖

```
just arch=linux-arm64 download-all          # arm64 下载
just arch=linux-arm64 docker-build          # arm64 镜像构建
PROXY_URL=http://proxy:8080 just download-all   # 代理覆写
just download-all 'http://proxy:8080'       # 位置参数覆写（优先级最高）
```

### 5.4 代理策略

| 来源 | 优先级 | 示例 |
|------|--------|------|
| 命令行位置参数 | 🔺 最高 | `just download-all 'http://myproxy:8080'` |
| 环境变量 `PROXY_URL` | 中 | `PROXY_URL=http://x just download-all` |
| `.env` 文件 | 🔻 最低 | `.env` 中写入 `PROXY_URL=http://192.168.10.1:10808` |
| 未设置 | 不使用代理 | `http_proxy=` 空，无 `--proxy` 参数 |

下载命令采用**双重代理保障**：
- **前置变量**：`http_proxy={{proxy_url}} https_proxy={{proxy_url}} curl ...`
- **显式参数**：`curl ... --proxy {{proxy_url}}`（部分 curl 版本 env proxy 可能不生效）

---

## 六、Docker 构建设计

### 6.1 双层镜像结构

```
base.dockerfile              Dockerfile
─────────────────────        ─────────────────────
FROM debian:trixie-slim      FROM builder-base:latest
COPY apt-sources/...         COPY bundle/binaries/${BUNDLE_ARCH}/...  → /cnb/lifecycle/
RUN apt-get update && \      COPY fnm/uv/uvx                         → /usr/local/bin/
    install ca-certificates  COPY docker/buildpacks/node/            → /cnb/buildpacks/
    curl unzip               COPY docker/buildpacks/python/          → /cnb/buildpacks/
                              COPY --from=procfile-bp (official)      → /cnb/buildpacks/
                              COPY order.toml                        → /cnb/order.toml
```

设计理由：
- **base 层**变化频率低（仅 apt 依赖），可单独缓存
- **builder 层**随 bundle 内容变化，独立迭代
- `just docker-build` 自动先构建 base，再构建最终镜像

### 6.2 构建参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `BASE_IMAGE` | `debian:trixie-slim` | base.dockerfile 的基础 OS |
| `BASE_IMAGE_TAG` | `offline-builder-base:latest` | 上一步构建的 base 镜像 tag |
| `PROCFILE_BP_TAG` | `paketobuildpacks/procfile:5.13.4` | 官方 Procfile buildpack 镜像 |
| `BUNDLE_ARCH` | `linux-amd64` | 架构，驱动 bundle COPY 路径 |

### 6.3 APT 镜像源注入

```dockerfile
# docker/apt-sources/sources.list 已预置在构建上下文中
COPY docker/apt-sources/sources.list /etc/apt/sources.list
RUN rm /etc/apt/sources.list.d/*
```

- 文件来源：宿主机 `/etc/apt/sources.list`（清华 / 阿里 / 中科大等镜像源）
- 预复制到 git 仓库，无需每次构建时同步
- 构建时直接替换容器内默认源，apt-get 走国内镜像

### 6.4 工具链位置

| 工具 | 用途 | 容器内路径 |
|------|------|-----------|
| lifecycle | CNB 构建核心 | `/cnb/lifecycle/` |
| fnm | Node 版本管理器 | `/usr/local/bin/fnm` |
| uv | Python 包管理器 | `/usr/local/bin/uv` |
| uvx | uv 工具执行器 | `/usr/local/bin/uvx` |

fnm/uv/uvx 放入 `/usr/local/bin/`（默认在 PATH 中），build 脚本通过 `PATH` 直接调用。

### 6.5 架构感知 COPY

```dockerfile
ARG BUNDLE_ARCH=linux-amd64
COPY bundle/binaries/${BUNDLE_ARCH}/lifecycle/ /cnb/lifecycle/
COPY bundle/binaries/${BUNDLE_ARCH}/fnm       /usr/local/bin/fnm
COPY bundle/binaries/${BUNDLE_ARCH}/uv        /usr/local/bin/uv
COPY bundle/binaries/${BUNDLE_ARCH}/uvx       /usr/local/bin/uvx
```

切换架构：
```bash
just arch=linux-arm64 docker-build
# → --build-arg BUNDLE_ARCH=linux-arm64
```

---

## 七、Buildpack 方案

### 7.1 三种 Buildpack

| Buildpack | ID | detect 条件 | build 行为 |
|-----------|----|-------------|-----------|
| Node | `paketo-buildpacks/node` | `package.json` 存在 | fnm install Node → npm install / yarn / pnpm |
| Python | `paketo-buildpacks/python` | `requirements.txt` 等存在 | uv venv → uv pip install 依赖 |
| Procfile | `paketo-buildpacks/procfile` | `Procfile` 存在 | 翻译为 Process Types |

### 7.2 Procfile Buildpack 来源

使用 **Paketo 官方 Procfile buildpack 镜像** 多阶段提取：

```dockerfile
FROM paketobuildpacks/procfile:5.13.4 AS procfile-bp
COPY --from=procfile-bp /cnb/buildpacks/paketo-buildpacks_procfile/5.13.4/ \
                       /cnb/buildpacks/paketo-buildpacks_procfile/5.13.4/
```

不自行实现 Procfile buildpack，直接复用官方二进制。

### 7.3 detect 顺序

`docker/order.toml`:
```toml
[[order]]                                   # 1. Node
  [[order.group]]
    id = "paketo-buildpacks/node"

[[order]]                                   # 2. Python
  [[order.group]]
    id = "paketo-buildpacks/python"

[[order]]                                   # 3. Procfile（兜底）
  [[order.group]]
    id = "paketo-buildpacks/procfile"
```

CNB lifecycle 按 order 依次 detect，第一个 match 的 buildpack 进入 build 阶段。

### 7.4 Buildpack 元数据

使用 Paketo 官方命名空间 `paketo-buildpacks/*`，遵循 CNB `api = "0.8"` 规范：

```toml
api = "0.8"
[buildpack]
  id = "paketo-buildpacks/node"
  version = "0.1.0"

[[targets]]
  os = "linux"
  arch = "amd64"

[[targets]]
  os = "linux"
  arch = "arm64"

[[stacks]]
  id = "io.buildpacks.stacks.bionic"
```

---

## 八、数据流与执行逻辑

### 8.1 下载阶段（`just download-all`）

```
download-all (依赖组合)
├── download-lifecycle
│   1. mkdir -p archives/{arch}  binaries/{arch}
│   2. curl --proxy → 下载 .tgz 到 archives/{arch}/
│   3. tar -xzf → 解压到 binaries/{arch}/lifecycle/
│
├── download-fnm
│   1. mkdir -p archives/{arch}  binaries/{arch}
│   2. curl --proxy → 下载 .zip 到 archives/{arch}/
│   3. unzip → 解压到 binaries/{arch}/ (根级 fnm 文件)
│   4. chmod +x
│
└── download-uv
    1. mkdir -p archives/{arch}  binaries/{arch}
    2. curl --proxy → 下载 .tar.gz 到 archives/{arch}/
    3. tar -xzf → 解压（内含 uv-{target}/ 目录）
    4. 提级：mv uv-{target}/{uv,uvx} → binaries/{arch}/
    5. rm -rf uv-{target}/
    6. chmod +x
```

### 8.2 Docker 构建阶段（`just docker-build`）

```
docker-build
├── docker-build-base
│   1. COPY apt-sources/sources.list → /etc/apt/sources.list
│   2. apt-get update && install ca-certificates curl unzip
│   3. tag → offline-builder-base:latest
│
└── docker-build (依赖 base)
    1. COPY bundle/binaries/${BUNDLE_ARCH}/lifecycle/ → /cnb/lifecycle/
    2. COPY fnm /usr/local/bin/fnm
    3. COPY uv /usr/local/bin/uv
    4. COPY uvx /usr/local/bin/uvx
    5. COPY docker/buildpacks/node/ → /cnb/buildpacks/paketo-buildpacks_node/0.1.0/
    6. COPY docker/buildpacks/python/ → /cnb/buildpacks/paketo-buildpacks_python/0.1.0/
    7. COPY --from=procfile-bp → /cnb/buildpacks/paketo-buildpacks_procfile/5.13.4/
    8. COPY order.toml → /cnb/order.toml
    9. tag → offline-builder:latest
```

### 8.3 运行时构建阶段（`pack build`）

```
pack build my-app --builder offline-builder:latest

1. detect   → 遍历 order.toml，match 项目类型（package.json / requirements.txt / Procfile）
2. analyze  → 读取之前构建的层缓存
3. build    → 执行选中 buildpack 的 bin/build：
   - 将 /usr/local/bin/ 下的 fnm/uv 加入 PATH
   - Node: fnm install → npm / yarn / pnpm
   - Python: uv venv → uv pip install
   - 生成 layers 到 /layers
4. export   → 打包 layers 为 OCI 镜像层
```

---

## 九、关键设计决策

### 9.1 架构映射

| 架构参数 | 通用名 | fnm | uv | Lifecycle |
|----------|--------|-----|----|-----------|
| `linux-amd64` | x86_64 | `linux` | `x86_64-unknown-linux-musl` | `linux.x86-64` |
| `linux-arm64` | aarch64 | `arm64` | `aarch64-unknown-linux-musl` | `linux.arm64` |

**不能**用一个统一的 `arch` 变量拼接所有 URL。每种工具的架构命名不同：
- fnm: `linux` / `arm64`（无统一前缀）
- uv: Rust musl target triple
- Lifecycle: 点号分隔 `linux.x86-64`

`config.just` 中为每种工具独立定义 `{lc,fnm,uv}_target` 变量。

### 9.2 容器运行时自动检测

```just
container_tool := if shell("command -v podman") != "" { "podman" } else { "docker" }
```

优先使用 podman（rootless、daemonless），降级到 docker。

### 9.3 为什么用 `just` 而不是 shell 脚本

- 内置变量覆盖：`just arch=linux-arm64 download-all`（无需编辑脚本）
- import 模块化：功能拆分到独立 .just 文件
- 依赖组合：`: recipe-a recipe-b` 声明式依赖
- `--dry-run` 预览执行计划
- 比 Makefile 更少隐式规则

### 9.4 为什么压缩包与二进制分离

- `archives/`：保留原始压缩包，可重新解压、校验、分发
- `binaries/`：解压后可执行文件，供 Docker build COPY
- 分离后 `archives/` 可以只分发一次，`binaries/` 按需重建

### 9.5 为什么拆分 base.dockerfile 和 Dockerfile

- base 层变化频率低（仅 apt 依赖），可独立缓存
- builder 层随 bundle/ 内容变化，单独迭代
- `just docker-build` 自动联动，对用户透明
- 单独构建 base：`just docker-build-base`

### 9.6 单一 Builder 镜像设计

与 Paketo 官方设计一致——一个 builder 镜像包含多个 buildpack：
- **不拆分成多个镜像**，Node / Python / Procfile 打包进同一个 Builder 镜像
- **运行时自动 detect**：CNB lifecycle 按 order 依次执行 `bin/detect`
- **优点**：一个镜像适配所有项目类型，减少镜像数量

---

## 十、限制与边界

| 限制 | 说明 |
|------|------|
| `just` 1.53+ 特性 | 依赖组合、import、private 变量 |
| Procfile buildpack | 使用 Paketo 官方镜像多阶段提取，不自实现 |
| SDK 版本 | 通过 `config.just` 集中管理，不自动追踪 latest |
| 网络 | 完全离线设计，所有组件预先下载 |
| base.dockerfile | 仅安装 ca-certificates / curl / unzip，其他运行时依赖在 buildpack 脚本中处理 |

---

## 十一、项目状态总览

```
阶段                         状态    说明
───────────────────────────────────────────────────────────────
scripts/ 模块拆分             ✅     5 个 .just + log.just
config.just                  ✅     版本/URL/代理/架构映射
download.just                ✅     3 个下载 recipe + download-all
docker.just                  ✅     docker-build / docker-build-base
info.just                    ✅     info / check-deps / list / clean
justfile (汇总入口)          ✅     仅 import + _default
base.dockerfile              ✅     apt 依赖 + 宿主 APT 源注入
Dockerfile                   ✅     multi-stage, BUNDLE_ARCH 参数化
docker/apt-sources/          ✅     预置清华镜像源
docker/order.toml            ✅     Node → Python → Procfile
Buildpack: node              ✅     detect + build + buildpack.toml
Buildpack: python            ✅     detect + build + buildpack.toml
Buildpack: procfile          ✅     官方 Paketo 镜像多阶段提取
bundle/ 实际下载              ⬜     需要真实网络运行（curl fnm/uv/lifecycle）
测试示例 (apps/)              ⬜     待创建
运行时缓存 (cache/)           ⬜     待设计
```
