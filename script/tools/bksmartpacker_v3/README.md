# bksmartpacker_v3（内部 fork）

把基于 `app_desc.yaml` 的应用构建成蓝鲸 PaaS3.0 的 S-Mart 包。

本目录是上游 `bksmartpacker_v3` 的 fork，不是原样拷贝：

- 上游来源与基线见 [NOTICE](NOTICE)
- 架构与执行链路见 [ARCHITECTURE.md](ARCHITECTURE.md)
- 与官方的功能差异见 [对比文档](../../../docs/design/components/bksmartpacker/vs-upstream.md)
- 维护与升级方式见 [fork 说明](../../../docs/design/components/bksmartpacker/fork.md)

## 一、能力概览

- 读取 `app_desc.yaml`，按模块生成 CNB 构建计划
- 在构建容器中执行 lifecycle（detect / build / export）
- 导出方式：
  - `layout`（默认）：容器内导出 OCI layout，打包为层包写回宿主
  - `daemon`：导出到 docker daemon，再由 `image save` 取回
- 运行时可选 docker 或 podman
- 支持指定目标平台（如 `linux/arm64`）
- 组装 `.Version=cnb-image-layers` 的 S-Mart 包
- `--inspect` 检查已有 S-Mart 包结构

## 二、构建

```bash
# 宿主架构
just --justfile script/tools/bksmartpacker_v3/justfile build

# 交叉编译
TARGET=linux-arm64 just --justfile script/tools/bksmartpacker_v3/justfile build
```

产物输出到 `script/bin/bksmartpacker-<os>-<arch>`，源码目录不入库二进制。

## 三、使用

> `--help` / `-h` 由 Kong 生成；`--inspect` 与 `--src` 互斥。

```bash
bksmartpacker \
  --src /path/to/app \
  --dest ./s-mart.tar.gz \
  --env-file project/offline-paketo-bk/build.env
```

`--src` 必须是包含 `app_desc.yaml` 的目录。多模块应用会逐个模块构建。

常用参数：

| 参数 | 说明 |
|---|---|
| `--src` | 应用源码目录（必填） |
| `--dest` | S-Mart 输出路径，默认 `./s-mart.tar.gz` |
| `--cache-registry` | 构建缓存镜像仓库 |
| `--registry-auth` | 仓库凭证（JSON） |
| `--env-file <path>` | dotenv 配置，可重复；按顺序覆盖 app_desc |
| `--packaging-version` | `v1` 或 `v2`，默认 `v2` |
| `--base-image-id` | artifact.json 的 runtime.base_image_id，默认 `default` |
| `--architecture` | artifact.json 的目标架构，默认按目标平台推断 |
| `--env KEY=VALUE` | 运行时覆盖，可重复；优先级最高 |
| `--module <name>` | 只构建指定模块，可重复；空表示全部；部分包不可部署 |
| `--allow-partial` | 允许 `--module` 生成部分调试包 |
| `--inspect <pkg>` | 只检查已有包，不构建 |
| `--inspect-format` | `text` 或 `json`，默认 `text` |

## 四、环境变量

| 变量 | 作用 |
|---|---|
| `BKS_CONTAINER_CLI` | 指定 `docker` 或 `podman`，缺省自动探测 |
| `BKS_DOCKER_SOCK` | 传给容器的 socket 路径 |
| `BKS_EXPORT_MODE` | `layout`（默认）或 `daemon` |
| `BKS_PLATFORM` | 目标平台，如 `linux/arm64` |
| `BKS_RUN_IMAGE_ARCHIVE_FILE` | 以文件形式提供 run image |
| `BKS_EXTRA_MOUNTS` | 追加 `--mount` 参数 |
| `BKS_NETWORK` | 传给 `docker run` 的网络模式 |
| `BKS_BUILDPACK_ALIASES` | 覆盖 buildpack 别名 |
| `BKS_LOG_LEVEL` | 日志级别：`debug`/`info`/`warn`/`error`，默认 `info` |

## 五、检查产物

```bash
script/bin/bksmartpacker-linux-amd64 --inspect dist/linux-amd64/s-mart.tar.gz
script/bin/bksmartpacker-linux-amd64 --inspect dist/s-mart.tar.gz --inspect-format json
```

输出包含 `path`、`size`、`.Version`、`app_desc` 是否存在、文件清单与层包摘要。

## 六、已知限制

- `daemon` 导出仅适用于真 dockerd；podman 的 docker 兼容层会让 lifecycle
  报 `singleton connection already in use`（issue #1763），podman 下请用 `layout`
- 上游对该工具没有 executor 级测试，升级后需要手工跑一次 amd64/arm64 打包

## 七、更多

- 架构说明：[ARCHITECTURE.md](ARCHITECTURE.md)
- 上游来源：[NOTICE](NOTICE)
