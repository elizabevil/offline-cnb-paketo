# offline-paketo-bk 测试

> 默认非 root builder。构建产物与校验和落在 `.cache/tests/`，不入库。

## 1. 完整命令

### 1.1 just 入口（推荐）

```bash
cd /home/debi/IdeaProjects/Learn-cnb-paketo

just --justfile project/offline-paketo-bk/justfile test-dind \
    app=project/offline-paketo-bk/fixtures/single \
    platform=linux-amd64
```

等价根目录转发：

```bash
just test-bk-dind app=... platform=...
```

### 1.2 直接调用脚本

```bash
BKS_TEST_MODE=dind \
BKS_TEST_EXPORT=layout \
BKS_APP=project/offline-paketo-bk/fixtures/single \
BKS_PLATFORM=linux-amd64 \
BKS_CACHE_DIR=.cache/bk-cache-run \
project/offline-paketo-bk/tests/managed.sh
```

### 1.3 四个推荐入口

```bash
just test-bk-quick          # host-podman + non-root + layout
just test-bk-pind           # pind + non-root + layout
just test-bk-dind           # dind + non-root + layout
just test-bk-dind-daemon    # dind + non-root + daemon
```

## 2. 参数

### 2.1 just 参数

| 参数 | 默认 | 说明 |
|---|---|---|
| `app` | `fixtures/single` | 应用目录，需含 `app_desc.yaml` |
| `platform` | 宿主架构 | `linux-amd64` 或 `linux-arm64` |

### 2.2 环境变量（脚本）

| 变量 | 默认 | 说明 |
|---|---|---|
| `BKS_APP` | `fixtures/single` | 应用目录 |
| `BKS_PLATFORM` | `linux-amd64` | 目标平台 |
| `BKS_TEST_MODE` | `pind` | `pind` 或 `dind` |
| `BKS_TEST_EXPORT` | `layout` | `layout` 或 `daemon` |
| `BKS_CACHE_DIR` | `.cache/bk-cache-run` | 跨构建缓存目录 |
| `BPS_BIN` | `script/bin/bksmartpacker-linux-amd64` | bksmartpacker 二进制 |
| `BKS_INNER_CONTAINER_CLI` | `/tmp/dockerbin/docker` | 仅 dind 使用，inner Docker CLI |
| `BKS_BUILDER_USER` | `1000:1000` | builder 用户 |
| `BKS_BUILDER_USERNS` | `keep-id` | builder userns |

## 3. 前置条件

- 目标 runtime 镜像已加载：

```bash
podman image exists offline-runtime-pind-bk:amd64
podman image exists offline-runtime-dind-bk:amd64
```

- builder 镜像已存在（`test-pind` / `test-dind` 会自动先 `smart-builder`）。
- dind 需要 inner Docker CLI 在 `/tmp/dockerbin/docker`（或通过 `BKS_INNER_CONTAINER_CLI` 指定）。

## 4. 脚本

| 文件 | 用途 |
|---|---|
| `quick.sh` | host-podman 快速验证 |
| `managed.sh` | pind/dind 非 root 验证 |
| `export-modes.sh` | host layout/daemon 对比 |
| `matrix.sh` | 矩阵入口 |

`managed.sh` 用 `/tmp/cnb-managed-*` 短路径作 workspace：Unix socket 路径超过
108 字符会连接失败。

## 5. 产物

```text
.cache/tests/offline-paketo-bk/<platform>/s-mart-<runtime>-<export>-<arch>.tar.gz
```

例：

```text
.cache/tests/offline-paketo-bk/linux-amd64/s-mart-dind-layout-amd64.tar.gz
```

## 6. 历史

原 `run-test.sh` 已迁移为 `quick.sh`。
