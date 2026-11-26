# 运行时前置概念：layout、daemon、pind/dind

> 读者：第一次接触本地 CNB 构建 / BK s-mart 打包的人。
> 目的：先统一概念，再看 [architecture.md](architecture.md) 和
> [build-chain.md](build-chain.md)。

## 1. 为什么需要这些概念

一次构建里同时存在多个“运行时”：

```text
宿主机运行时
  └─ builder 容器
       └─ lifecycle
            └─ 可选的嵌套运行时（pind/dind）
```

它们负责的事情不同。混在一起会导致：

- 不知道镜像被写到了哪里
- 不知道缓存属于哪一层
- 把 buildpack cache 和 runtime image cache 混为一谈
- 遇到 `singleton connection already in use` 时无法定位

## 2. 三个镜像概念

### 2.1 builder 镜像

用于执行 detect/build 的镜像，包含：

```text
lifecycle
buildpacks
cnb CLI
基础工具
```

它不负责最终应用的运行。

### 2.2 run image

最终应用镜像的**基础运行层**。

BK 流程为了只输出应用层，使用一个单层的 fake runner：

```text
fake-cnb-runner:latest
```

它不是真实 OS，只是 exporter 需要的基础镜像占位。

### 2.3 应用镜像

exporter 产出的结果：

```text
run image
+ app layers
+ launcher
+ metadata
```

BK 最终只把应用层打进 s-mart。

## 3. outer / inner runtime

### outer runtime

bksmartpacker 直接调用的宿主 Podman/Docker：

```text
podman run builder
podman run runtime sidecar
podman image import/save
podman volume
```

### inner runtime

builder 容器通过 socket 访问的运行时：

```text
builder
  /var/run/docker.sock
    → runtime-pind  → inner Podman
    → runtime-dind  → inner Dockerd
```

### host / pind / dind

| 名称 | outer | inner | 典型用途 |
|---|---|---|---|
| host | 宿主机 | 宿主机 | 本机已有运行时 |
| pind | 宿主机 | sidecar Podman | 隔离、Podman 环境 |
| dind | 宿主机 | sidecar Dockerd | 需要 Docker daemon |

outer 和 inner **不是同一个概念**：

```text
outer Podman + inner Podman  → 允许
outer Podman + inner Dockerd → 允许
outer Docker + inner Podman  → 允许
```

## 4. 导出模式

`CNB_EXPORT_MODE` 决定 exporter 把镜像写到哪里。

### 4.1 layout

```text
exporter -layout
  → /layers/oci 下的 OCI layout 目录
  → bksmartpacker 读取并打包层
```

特点：

- 不需要 daemon
- 不依赖 Docker API
- pind 默认
- 可以避开 lifecycle singleton

### 4.2 daemon

```text
exporter -daemon
  → 写入 inner runtime
  → bksmartpacker 通过 image save 导出
```

特点：

- 需要可用的 Docker-compatible API
- dind 使用
- 受 lifecycle `priv.DockerClient` 单连接限制影响

### 4.3 none

```text
只执行 detect/build/analyze
不导出镜像
```

用于探测或只验证前三个阶段。

## 5. layout 与 daemon 的产物差异

| 项 | layout | daemon |
|---|---|---|
| 镜像位置 | 文件系统 OCI layout | inner runtime image store |
| 是否需要 socket | 否 | 是 |
| 是否需要 image save | 由 bksmartpacker 处理 layout | 需要 |
| pind | 默认 | 不推荐 |
| dind | 可用但无 inner image | 推荐 |
| singleton 风险 | 无 | 有 |

## 6. singleton connection already in use

来源：

[buildpacks/lifecycle#1763](https://github.com/buildpacks/lifecycle/issues/1763)

条件：

```text
lifecycle exporter -daemon
Podman Docker-compatible API
socket 在容器内属于 root
```

根因：

```text
lifecycle priv/docker.go 复用单条预打开连接
第二个请求无法 dial，直接报错
```

已确认：

- Podman 服务端并发 `image get` 正常
- singleton 是 lifecycle 客户端问题
- `layout` 可以绕过

非 root builder：

```text
--user 1000:1000
--userns=keep-id
```

可以让 socket 在容器内显示为非 root，从而避开
`priv.shouldConnectSock()` 的 singleton 分支。

## 7. 推荐选择

```text
pind:
  layout

dind:
  默认 daemon
  也可显式 layout（不经过 inner dockerd 的 image store）
  + non-root builder（daemon 路径）
  + keep-id（daemon 路径）
  + 可写 workspace/cache

cnb + Podman daemon:
  优先 layout
  必须 daemon 时才承担 #1763 风险
```

## 8. 缓存边界

| 缓存 | 位置 | 负责谁 |
|---|---|---|
| buildpack cache | `/layers/cache` | buildpacks |
| runtime image cache | `/cnb/runtime` 命名卷 | inner runtime |
| workspace | `/cnb/work` | 单次构建 |

layout 模式下 inner runtime 通常没有镜像，因此：

```text
runtime cache state=cold
```

不代表 buildpack cache 未命中。

## 9. 测试矩阵

各组合的实测/推测结果见：

[runtime-matrix.md](components/bksmartpacker/runtime-matrix.md)


## 10. 多架构

runtime-pind 已支持按目标平台构建：

```bash
BKS_PLATFORM=linux/arm64 just runtime-pind-bundle
```

使用：

```text
arm64 static podman
arm64 Debian base
```

runtime-dind 也使用静态 Docker asset；amd64/arm64 都不依赖 Docker 基础镜像。


## 11. dind 依赖清单

必需：

```text
Debian base
静态 Docker 物料：
  dockerd
  containerd
  runc
  docker
  docker-proxy
  docker-init
  containerd-shim-runc-v2
```

当前默认 rootless 配置不需要 iptables：

```text
--bridge=none
--iptables=false
--ip-forward=false
```

如果启用 rootful bridge 网络，还需要：

```text
iptables/nftables
iproute2
```

当前静态物料不包含这些额外系统包。


## 12. arm64 pind 与 qemu

`qemu-user` 模拟 arm64 时，Podman 默认 robust shared-memory lock 会失败：

```text
failed to create 2048 locks in /libpod_lock: function not implemented
```

原因是 qemu-user 不完整支持 robust futex / `set_robust_list` 路径。
这不是 arm64 Podman 本身的缺陷。

当前决策：

```text
arm64 pind 交给真机或 arm64 CI 验证
qemu-user 下改用 lock_type=file 仅作为临时探测手段
```

`qemu-system` 需要完整 arm64 虚拟机和 rootfs，暂不纳入当前测试链。
