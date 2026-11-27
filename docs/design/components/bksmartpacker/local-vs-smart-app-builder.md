# 当前 bksmartpacker_v3 vs 官方 smart-app-builder

> 对比对象：
> - 本地：`script/tools/bksmartpacker_v3`
> - 官方：blueking-paas `builder-stack` 的 `smart-app-builder`
>
> 本文对应当前代码，而不是早期 D1/D6 阶段。

## 1. 结论

本地已经**吸收官方 v2 的核心语义**：

- 构建组
- 应用级 Procfile
- `<module>-<process>` 命名空间
- `artifact.json`
- 一个构建组一个 image tar
- 包体积约 138 MB

仍然保留本地化差异：

- 本地路径输入/输出，而不是 `SOURCE_GET_URL` / `DEST_PUT_URL`
- 本地 `cnb lifecycle run`，而不是 `cnb-builder-shim`
- fake run image，而不是真实 `CNB_RUN_IMAGE`
- 本地缓存，而不是远程 `CACHE_REGISTRY`
- buildpack 只支持内置 `oci-embedded`
- 不实现完整的 pind/dind 编排，只支持运行时配置

## 2. 当前实现快照

```text
输入：app_desc.yaml + 源码目录
默认打包：v2
构建单位：sourceDir + buildpacks 相同的模块组
Procfile：<module>-<process>
输出：每组一个 <group>.tar
映射：artifact.json
压缩：s-mart.tar.gz
```

SOPS 四模块实测：

```text
1 次 builder build
1 个 image tar（default.tar）
18 个 namespaced process
138 MB
```

## 3. 入口与参数

| 项 | 当前 bksmartpacker | 官方 smart-app-builder |
|---|---|---|
| 源码 | `--src` 本地目录 | `SOURCE_GET_URL` |
| 产物 | `--dest` 本地路径 | `DEST_PUT_URL` |
| 打包版本 | `--packaging-version v1\|v2`，默认 v2 | `PACKAGING_VERSION`，默认 v2 |
| builder 镜像 | `--builder-image` | `BUILDER_SHIM_IMAGE` |
| run image | 固定 fake runner | `CNB_RUN_IMAGE` / `CNB_RUN_IMAGE_TAR` |
| base image | `--base-image-id`，默认 default | `BASE_IMAGE_ID` |
| 架构 | `--architecture`，默认按平台推断 | `ARCHITECTURE` |
| 构建变量 | `--env-file` / `--env` | 固定环境 + 平台注入 |
| 缓存 | `--cache-registry`，可选 | `CACHE_REGISTRY` |
| 运行时 | `BKS_RUNTIME` / `RUNTIME` | `RUNTIME=pind\|dind` |
| CLI | Kong 结构体 | 环境变量驱动 |

当前 CLI 由 `pkg/cli` 负责：

```text
pkg/cli/options.go   Options / Parse / Validate / Params
pkg/cli/inspect.go   s-mart 包检查输出
```

## 4. 构建单位

### 当前

```text
sourceDir + buildpacks 相同 → 一个构建组
组内只构建一次
组内模块共享同一个 image tar
```

代码位置：

```text
pkg/plan/new.go       生成 Steps
pkg/plan/groups.go    构建组
pkg/plan/procfile.go  应用级 Procfile
```

### 官方

同样按构建组：

```text
sourceDir + required buildpacks → 构建组
一个构建组 → 一个镜像
一个镜像 → 一个 <group>.tar
```

这一点已经对齐。

## 5. Procfile 与进程

当前生成：

```yaml
default-web: bin/start_web.sh
default-beat: python manage.py celery beat -l info
pipeline-cworker: python manage.py celery worker ...
pipeline-node-timeout: python manage.py node_timeout_process
```

artifact.json：

```json
{
  "runtime": {
    "base_image_id": "default",
    "architecture": "amd64"
  },
  "app_artifacts": {
    "pipeline": {
      "image_tar": "default.tar",
      "proc_entrypoints": {
        "cworker": ["pipeline-cworker"],
        "node-timeout": ["/cnb/process/pipeline-node-timeout"]
      }
    }
  }
}
```

与官方一致：

- 进程名全局唯一
- 平台按 `proc_entrypoints` 设置容器 command
- 本地生成绝对路径 `/cnb/process/<module>-<process>`，避免依赖运行时 PATH
- 不再直接执行裸 `node-timeout` / `cworker`

## 6. 产物

### 当前 v2

```text
s-mart.tar.gz
├── .Version
├── app_desc.yaml
├── logo.png
├── artifact.json
└── default.tar
```

### 当前 v1（兼容模式）

```text
s-mart.tar.gz
├── .Version
├── app_desc.yaml
├── logo.png
└── <module>.tgz
```

v1 仅用于旧平台/旧产物验证，不再默认。

### 包体积

SOPS：

```text
v1 每模块 tgz：约 549 MB
D1 去重实验：约 136 MB
当前 v2：约 138 MB
```

当前 v2 不依赖 D1/D6 脚本。

## 7. 执行器与运行时

### 当前模型

```text
Backend:      docker | podman
Environment:  host | pind | dind
```

组合规则：

| Backend | Environment | 用途 |
|---|---|---|
| docker | host | 本地 docker |
| podman | host | 本地 podman |
| podman | pind | podman-in-docker 的 socket 预设 |
| docker | dind | docker-in-docker |

代码位置：

```text
pkg/container/config.go    环境变量解析、socket、export mode
pkg/builder/executors/     fake runner、build、export、package
```

### 与官方差异

| 项 | 当前 | 官方 |
|---|---|---|
| 构建入口 | `cnb lifecycle run` | `cnb-builder-shim` |
| 导出 | OCI layout，必要时 daemon | docker save / daemon |
| run image | `fake-cnb-runner`；可扩展自定义 run image | 真实 `CNB_RUN_IMAGE` |
| 平台 API | `0.11` | 同 |
| 缓存 | 本地目录 | `CACHE_REGISTRY` |
| 运行时 | host docker/podman；pind/dind 配置预设 | 完整 pind/dind 环境 |
| 进程入口 | artifact.json | artifact.json |

说明：

- 当前 `pind` 只负责选择 podman 后端和 rootful socket，不负责启动嵌套 Podman。
- 当前 `dind` 只负责 docker 后端和 daemon 导出，不负责启动 Docker daemon。

## 8. 平台处理

### 当前 v2

平台要求 apiserver ≥ 1.7：

```text
读 artifact.json
按模块取 image_tar
按进程取 proc_entrypoints
容器 command = <module>-<process>
```

### 旧 CNB

```text
按模块找 <module>.tgz
读取 tar 中第一个 manifest
进程名直接使用
```

旧模式的问题：

- 多镜像 tar 只取第一个
- 同名 `web` 冲突
- 缺进程时直接 `exec` 失败

当前默认 v2 已规避。

## 9. 官方能力中未吸收的部分

| 能力 | 当前状态 | 原因 |
|---|---|---|
| `cnb-builder-shim` | 未使用 | 本地直接调用 lifecycle |
| `SOURCE_GET_URL` / `DEST_PUT_URL` | 未使用 | 本地文件路径足够 |
| 真实 `CNB_RUN_IMAGE` | 未使用 | 本地 fake runner 降低依赖 |
| 完整 pind/dind | 未实现 | 只做运行时配置预设 |
| `RUNTIME_WORKSPACE` | 未使用 | 本地 workspace 由 executor 管理 |
| `BUILDPACK_TYPE` 多形态 | 仅内置 oci-embedded | 本地 buildpack 已嵌入 builder |
| 远程缓存 registry | 可选，未作为主路径 | 离线优先 |
| 多语言官方 buildpack 全集 | 只保留 Go/Node/Python | 当前业务范围 |

## 10. 当前仍需注意

1. **平台版本**：v2 需要 apiserver ≥ 1.7。
2. **部分包**：`--module` 调试包不可部署，必须配合 `--allow-partial`。
3. **架构**：`--architecture` 必须与实际镜像一致；默认按 `BKS_PLATFORM` 或宿主推断。
4. **D1/D6**：仍保留历史脚本，但 v2 路径不再依赖。
5. **pind**：当前是配置预设，不是完整嵌套运行时。

## 11. 一句话总结

```text
当前 bksmartpacker_v3
  = 本地 lifecycle 执行器
  + 官方 v2 构建组/Procfile/artifact 语义
  + 本地 docker/podman 运行时
```

在最关键的产物协议和进程模型上已经与官方 v2 对齐；剩下的差异主要是本地离线执行方式，而不是包格式。
