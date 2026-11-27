# runtime image 使用场景与缓存需求

> 状态：需求与架构分析，dind 实现前的决策依据。

## 1. 背景

managed pind/dind 引入了 sidecar runtime。当前 runtime cache 观测显示：

```text
第一次构建: state=cold before_images=0 after_images=0
第二次构建: state=cold before_images=0 after_images=0
```

现象不是 sidecar 被提前结束，而是当前 layout 构建路径没有把镜像写入 inner runtime。

因此需要先明确：

- runtime image 在哪里被使用
- 哪些内容值得缓存
- layout 与 daemon 模式的缓存对象分别是什么

## 2. 两套运行时

### 2.1 outer runtime

bksmartpacker 直接调用的宿主 podman/docker：

```text
podman run builder
podman image import fake runner
podman image save 输出层
podman run runtime sidecar
podman volume 管理
```

相关实现：

```text
pkg/builder/executors/executor.go
pkg/builder/executors/sidecar.go
```

### 2.2 inner runtime

sidecar 内的 podman/dockerd，只通过 socket 暴露给 builder：

```text
builder
  /var/run/docker.sock
    → runtime-pind/runtime-dind
      → inner runtime
```

当前 builder 环境同时设置：

```text
USE_DOCKER_DAEMON=1
DOCKER_HOST=unix:///var/run/docker.sock
```

## 3. runtime image 使用场景

| 场景 | 运行时 | 产生 inner image |
|---|---|---|
| 启动 builder 容器 | outer | 否 |
| fake runner 构造/导入 | outer | 否 |
| run image archive 导出 | outer | 否 |
| layout exporter | 文件系统 | 否 |
| daemon exporter | inner | 是 |
| lifecycle 拉取 run image | 取决于配置 | 可能 |
| buildpack 显式调用 Docker | inner | 是 |
| buildpack 进程执行 | builder 内 | 否 |
| 输出层 image save | outer | 否 |
| 自定义 run image 导入 | outer/文件 | 视实现 |

当前 layout 链路：

```text
outer 启动 builder
→ lifecycle 在 builder 内执行 buildpacks
→ exporter 写 /layers/oci
→ bksmartpacker 用 outer image save 导出
```

inner runtime 不参与镜像生产，因此 inner image store 为空。

## 4. 缓存对象

### 4.1 高价值

| 内容 | 位置 | 说明 |
|---|---|---|
| buildpack cache | `/layers/cache` | 跨构建复用，优先级最高 |
| builder image | outer storage | 避免重复 pull |
| run image 基础层 | outer storage | 稳定基础层 |
| runtime-pind/dind image | outer storage | 镜像本身稳定 |

### 4.2 条件性价值

| 内容 | 位置 | 条件 |
|---|---|---|
| inner run image | inner storage | daemon exporter / 自定义 run image |
| inner builder image | inner storage | inner runtime 需要运行镜像 |
| inner buildpack images | inner storage | buildpack 显式依赖 Docker |
| inner output image | inner storage | daemon exporter |
| inner container layers | inner storage | 基本不复用 |
| inner volumes | inner volume | 生命周期短 |

### 4.3 低价值

| 内容 | 原因 |
|---|---|
| inner podman DB | 无镜像时只有元数据 |
| inner 容器可写层 | 每次构建不同 |
| 输出应用层 | 源码/版本变化后不可复用 |
| fake runner | 体积很小，收益低 |

## 5. 当前观测的含义

layout 模式：

```text
runtime volume 保留
但没有 inner image
```

所以当前 `runtime cache` 是诊断指标，不是性能指标。

buildpack cache 与 runtime image cache 必须分开：

```text
/layers/cache          buildpack 层缓存
cnb-runtime-<backend>  inner runtime 镜像缓存
```

## 6. 设计决策

### 6.1 layout 模式

保持：

```text
runtime storage 卷
runtime cache 观测
```

但不把 inner image cache 作为优化目标。

优先优化：

1. buildpack cache 命中
2. outer runtime 镜像复用
3. fake runner 复用

### 6.2 daemon 模式

只有 daemon exporter 或自定义 run image 场景才值得持久化 inner images。

需要明确：

- 谁往 inner runtime 写入镜像
- 谁从 inner runtime 导出层
- 镜像 tag/digest 的生命周期
- 清理策略

### 6.3 dind

dind 使用 inner dockerd，inner image 缓存价值高于 layout pind：

- 基础镜像
- build 中间镜像
- 输出镜像
- buildpack 显式构建的镜像

dind 的缓存卷、清理和并发隔离需要单独设计。

## 7. 缓存卷策略

当前：

```text
cnb-runtime-podman
cnb-runtime-docker
→ /cnb/runtime
```

清理：

```bash
podman volume rm -f cnb-runtime-podman
```

后续需要：

- 按 arch 拆卷
- 并发构建锁
- 保留/清理策略
- 卷大小上限

## 8. 非目标

本阶段不做：

- 为 layout 模式强行保留 inner images
- 自动 prune
- 多架构卷拆分
- daemon exporter 的完整缓存复用

## 9. 后续检查项

1. buildpack cache 命中率
2. outer builder/run image 复用
3. fake runner 是否重复创建
4. dind inner image cache
5. runtime cache 报告是否与构建模式匹配
