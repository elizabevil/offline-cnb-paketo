# Outer/Inner Runtime 测试矩阵

> 状态：随 outer/inner runtime 分离逐步更新。
> 环境：Linux amd64，rootless podman 5.4.2；当前测试机没有 Docker CLI/daemon。

## 1. 术语

| 名称 | 含义 |
|---|---|
| Outer runtime | bksmartpacker 用来启动 builder、sidecar、执行 image import/save 的运行时 |
| Inner runtime | builder 容器通过 socket 访问的构建运行时 |
| host | outer 和 inner 都使用宿主机运行时 |
| pind | inner 使用 sidecar 内的 podman service |
| dind | inner 使用 sidecar 内的 Docker daemon |

## 2. 测试矩阵

| ID | Outer | Inner | Driver/Environment | 状态 | 依据/备注 |
|---|---|---|---|---|---|
| A | Podman | Podman | host | **已验证** | 当前 host SOPS 构建通过 |
| B | Docker | Docker | host | 未测 | 本机无 Docker CLI/daemon |
| C | Docker | Podman | managed pind | 未测 | 推测可行，取决于 Docker 是否允许 privileged sidecar |
| D | Podman | Podman | managed pind | **已验证** | 当前 managed pind SOPS 构建通过 |
| E | Podman | Dockerd | managed dind | **已验证** | 静态 docker 物料；非 root builder + 可写 cache；CMDB 完整构建成功 |
| F | Docker | Dockerd | managed dind | 未测 | 推测可行，需 Docker 外层 + privileged |
| G | Docker | Dockerd | host dind | 未测 | 需要宿主机 Docker daemon |
| H | Podman | Podman Docker API | cnb 原始复现：daemon export | **上游 issue 已确认；本机完整 cnb 未复现** | https://github.com/buildpacks/lifecycle/issues/1763；fake root-owned socket 触发 |
| I | Podman | Podman Docker API | cnb 非 root 复现 | **已验证** | CMDB SaaS + podman daemon export 成功；非 root builder |
| J | Podman | Podman | managed pind arm64 | **暂缓** | runtime-pind arm64 镜像已生成；qemu-user robust futex 限制，交由真机/arm64 CI 验证 |
| K | Podman | Dockerd | managed dind arm64 | **已验证** | 静态 Docker arm64 asset + Debian arm64 base；CMDB arm64 完整构建成功 |

## 3. 推测说明

### B：host Docker

推测：

```text
成功
```

理由：

- 当前 host podman 只依赖 Docker-compatible CLI
- 需要 Docker daemon 与 socket
- image save/import 语义有差异，需要实际回归

风险：

- `image save` 默认格式与 podman 不同
- 运行用户/权限差异
- socket 路径和 API 版本差异

### C：outer Docker + inner Podman

推测：

```text
大概率成功
```

理由：

- outer Docker 负责启动 builder 与 runtime-pind sidecar
- sidecar 内是独立 podman
- builder 只通过 socket 访问 inner podman
- outer 不需要理解 inner 的 libpod API

风险：

- Docker 对 privileged sidecar 的限制
- 挂载路径和 rootless/rootful 差异
- outer 的 image save 不能替代 inner image save

### E：outer Podman + inner Dockerd

当前已验证：

```text
runtime-dind sidecar 启动成功
Docker API 27.5.1
storage-driver=vfs
run image 可加载到 inner Docker
```

root builder 会触发 lifecycle 已知问题：

```text
failed to fetch base layers: ... singleton connection already in use
```

非 root builder（`BKS_BUILDER_USER=1000:1000` +
`BKS_BUILDER_USERNS=keep-id`）后构建成功，说明 singleton 路径与 root
socket 客户端绑定有关。

前提：

- builder user 对 workspace/cache 可写
- inner docker socket 对 builder user 可访问

该构建的 builder entrypoint 虽然是 `/cnb/tools/cnb lifecycle run`，
但这是 **Dockerd** 路径，不等于 cnb 原始的 Podman Docker-compatible API 复现。
两者共享 lifecycle singleton 机制，但不能互相替代验证。

风险：

- outer Podman 启动 builder 时路径是宿主路径
- inner Dockerd 看到的路径是 sidecar 视角
- workspace/cache 路径需要映射
- inner image save 需要单独的 Docker client
- rootless 下 bridge/iptables 受限

### F：outer Docker + inner Dockerd

推测：

```text
成功概率较高
```

但需要：

- Docker outer daemon
- privileged sidecar
- inner Dockerd 网络/iptables 权限
- 双 Docker 的 storage/network 隔离

## 3.1 cnb 原始复现到底测什么

本地复现器：

```text
script/tools/cnb/tests/lifecycle-singleton
```

它使用 lifecycle 的 `priv.DockerClient` 和一个 root-owned Unix socket，
模拟的是一般 Docker-compatible API，不限定 dockerd。

实际 cnb 场景是：

```text
Outer: Podman
Inner: Podman Docker-compatible API
阶段: exporter daemon
条件: lifecycle 以 root 运行，socket 在容器内属于 root
```

不是：

```text
managed dind + dockerd
```

managed dind 的非 root 成功只能证明：

```text
lifecycle singleton 条件可以通过非 root + 非 root-owned socket 绕过
```

不能直接等同于：

```text
cnb + inner podman daemon export 已验证
```

cnb 测试目录中的复现器已执行：

```text
非 root:
ImageSave error: <nil>

podman unshare（root userns）:
ImageSave error: ... singleton connection already in use
```

这验证了 singleton 的触发条件。后续又做了完整 cnb 测试：

- 非 root + Podman daemon + CMDB：成功
- rootful Podman + root builder + CMDB：成功，未复现 #1763

因此当前结论是：

```text
复现器条件成立
完整 cnb root-owned 路径未复现
```

## 3.2 cnb + Podman daemon export 实际测试

测试入口：

```text
/cnb/tools/cnb lifecycle run
```

这是 builder 镜像的 entrypoint，不是 bksmartpacker 的模拟。

### 非 root builder

配置：

```text
Podman Docker-compatible socket
CNB_EXPORT_MODE=daemon
非 root + keep-id
```

结果：

```text
Saving bk_sops/pind-nonroot3:latest...
*** Images (5e1b0f510d0e)
```

daemon export 成功，没有 singleton。

### root builder

同样配置下也成功，但本机 rootless podman socket 在 builder 内
显示为非 root 属主，因此没有触发 singleton。

所以：

```text
本机 root 测试不能证明 root-owned socket 场景
```

root-owned socket 场景仍由：

```text
script/tools/cnb/tests/lifecycle-singleton
```

单独复现。

### rootful Podman 完整测试

又用 rootful Podman socket 跑了：

```text
rootful podman system service
root builder
root-owned socket
cnb lifecycle run
CNB_EXPORT_MODE=daemon
```

结果：

```text
Saving bk_sops/rootful-root3:latest...
*** Images (f30395cbd0a7)
```

**没有复现 singleton**。

### CMDB SaaS 完整复测

使用：

```text
rootful Podman
root-owned socket
root builder
cnb lifecycle run
CMDB SaaS release-amd64
CNB_EXPORT_MODE=daemon
DOCKER_API_VERSION=1.41
```

结果：

```text
Saving bk_cmdb_saas/web:rootful-api...
*** Images (93af80d0d29c)
```

**仍然没有复现 singleton。**

因此当前 cnb lifecycle 路径下，root-owned socket 条件还不足以单独触发。
上游 issue #1763 已确认该问题存在，但本机完整 cnb 路径没有复现；需要继续
定位 exporter 实际使用的 Docker 客户端路径。不能宣称“已复现”。

上游 workaround：

```text
使用 exporter -layout，而不是 -daemon
```

参考：

https://github.com/buildpacks/lifecycle/issues/1763

## 3.3 Podman 兼容 API 直接导出探测

脚本：

```text
script/tools/cnb/tests/podman-export-probe.sh
```

结果：

```text
by-tag size=17408
by-id  size=17408
并发 p1=0 size=17408
并发 p2=0 size=17408
```

说明：

```text
Podman 服务端本身允许顺序/并发 image get
singleton 不是 Podman API 的限制
```

singleton 来自 lifecycle 的 `priv.DockerClient` 客户端单连接实现。

## 3.4 多架构

已构建：

```text
bundle/linux-arm64/images/runtime-pind.tar
```

使用本地 arm64 Debian base：

```text
swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/debian:trixie-slim-linuxarm64
```

静态 podman 使用：

```text
podman-linux-arm64.tar.gz
```

验证边界：

```text
arm64 镜像构建：通过
qemu 下 arm64 pind sidecar：libpod robust futex 限制，暂不验证
arm64 dind：静态 Docker arm64 asset，CMDB arm64 完整构建成功
```

因此：

```text
pind arm64：产物已生成，交由真机/arm64 CI 验证
dind arm64：完整构建已验证
```

dind arm64 产物：

```text
/tmp/cnb-arm-dind-go/s-mart-airm.tar.gz
31 MB
sha256: 37bdf633f7032292cafc79fe38b3e4e119fe72d2cde58c470648b72545b07d6f
```

## 3.5 dind 静态依赖与完整验证

runtime-dind 不依赖 `docker:27-dind` 镜像，使用现有 `docker` 物料：

```text
bundle/<platform>/binaries/docker/27.0.3/docker/
  dockerd
  containerd
  runc
  docker
  docker-proxy
  docker-init
  containerd-shim-runc-v2
```

基础镜像：

```text
Debian trixie slim
```

完整验证：

```text
Outer Podman
Inner static dockerd
CMDB SaaS release-amd64
non-root builder
CNB_EXPORT_MODE=daemon
```

结果：

```text
Saving bk_cmdb_saas/web:3.14.8-alpha3.2...
*** Images (10a3b7ec0e96)
BuildSMartPackage done
```

产物：

```text
/tmp/cnb-dind-static-build/s-mart-dind.tar.gz
33 MB
sha256: eb2c3ac9b3da2be97e8581ac488422fc46d870928402ac6a35ae72e9232e71ab
```

当前无需额外依赖。

### 可选依赖

当前默认：

```text
--bridge=none
--iptables=false
--ip-forward=false
```

如果 rootful 环境需要 inner 容器桥接网络，才需要额外安装：

```text
iptables/nftables
iproute2
```

这些不在当前静态 Docker 物料内。

## 4. 当前结论

- host podman、outer podman + inner podman 已验证
- outer podman + inner dockerd 已验证（非 root builder），但这是 dind 路径
- cnb 原始 Podman Docker-compatible API daemon export 仍未做非 root 验证
- 外层 Docker 场景全部未测
- 不能因为 sidecar 启动成功就认为完整 dind 构建成功

## 5. 后续验证顺序

1. outer Docker + inner Docker（host）
2. outer Docker + inner Podman（managed pind）
3. outer Podman + inner Dockerd 的完整构建
4. 多架构重复上述矩阵

## 测试产物矩阵

组合定义：

```text
架构（amd64 / arm64）
× 运行时（host / pind / dind）
× 导出模式（layout / daemon）
= 12 个组合
```

外层工具不是独立维度，但必须写明：

| 运行时 | 外层工具 | 内层运行时 |
|---|---|---|
| host | podman 或 docker | 同外层 |
| pind | podman | sidecar podman |
| dind | podman | sidecar docker |

`dind` 默认仍是 daemon；显式传入 `BKS_EXPORT_MODE=layout` 时走 OCI layout 路径。

### 12 个组合

| 架构 | 运行时 | 外层工具 | 内层运行时 | 导出 | 合法性 | 产物 | 状态 | 原因 |
|---|---|---|---|---|---|---|---|---|
| amd64 | host | podman | 宿主 podman | layout | 合法 | `s-mart-host-layout-amd64.tar.gz` | 已生成 | 当前 host 路径 |
| amd64 | host | docker | 宿主 docker | daemon | 合法 | `s-mart-host-daemon-amd64.tar.gz` | 已生成 | 宿主机 Docker daemon |
| amd64 | pind | podman | sidecar podman | layout | 合法 | `s-mart-pind-layout-amd64.tar.gz` | 已生成 | inner Podman |
| amd64 | pind | podman | sidecar podman | daemon | 合法 | `s-mart-pind-daemon-amd64.tar.gz` | 已生成 | Podman Docker-compatible API |
| amd64 | dind | podman | sidecar docker | layout | 合法 | `s-mart-dind-layout-amd64.tar.gz` | 已生成 | layout 不经过 inner dockerd |
| amd64 | dind | podman | sidecar docker | daemon | 合法 | `s-mart-dind-daemon-amd64.tar.gz` | 已生成 | inner Docker daemon |
| arm64 | host | podman | 宿主 podman | layout | 合法 | `s-mart-host-layout-arm64.tar.gz` | 已生成 | 当前 host 路径 |
| arm64 | host | docker | 宿主 docker | daemon | 合法 | — | 未打包 | lifecycle singleton #1763 |
| arm64 | pind | podman | sidecar podman | layout | 合法 | — | 未打包 | qemu-user robust lock |
| arm64 | pind | podman | sidecar podman | daemon | 合法但受限 | — | 未打包 | qemu-user robust lock |
| arm64 | dind | podman | sidecar docker | layout | 合法 | `s-mart-dind-layout-arm64.tar.gz` | 已生成 | layout 不经过 inner dockerd |
| arm64 | dind | podman | sidecar docker | daemon | 合法 | `s-mart-dind-daemon-arm64.tar.gz` | 已生成 | inner Docker daemon |

### 当前已生成

```text
.cache/tests/full/linux-amd64/
  s-mart-host-layout-amd64.tar.gz
  s-mart-host-daemon-amd64.tar.gz
  s-mart-pind-layout-amd64.tar.gz
  s-mart-pind-daemon-amd64.tar.gz
  s-mart-dind-layout-amd64.tar.gz
  s-mart-dind-daemon-amd64.tar.gz

.cache/tests/full/linux-arm64/
  s-mart-host-layout-arm64.tar.gz
  s-mart-dind-layout-arm64.tar.gz
  s-mart-dind-daemon-arm64.tar.gz
```

共 9 个。

### 未生成原因汇总

| 原因 | 影响组合 | 类型 |
|---|---|---|
| arm64 host daemon 触发 lifecycle singleton #1763 | arm64 host + daemon | 上游缺陷 |
| qemu-user robust lock 限制 | arm64 pind × 2 | 验证环境限制 |

### 配置约束

`pkg/container/config.go`：

```text
daemon + 非 Docker → 仅 pind 例外（使用 Podman Docker-compatible API）
```

`dind + layout` 现在被允许：

```text
dind 默认        → daemon
dind + 显式 layout → OCI layout
```

### dind layout 的语义

```text
dind + layout
  → 启动 dind sidecar
  → builder 使用 OCI layout exporter
  → 不经过 inner dockerd 的 image store
```

它验证的是：

```text
dind 外层环境 + lifecycle layout 路径
```

不是：

```text
dind 的 docker image save 路径
```

后者由 `dind + daemon` 覆盖。

### 导出格式差异

```text
host/pind/dind + layout → OCI layout，gzip blob
dind + daemon           → docker-archive，未压缩 layer.tar
```

因此 `s-mart.tar.gz` 大小不能直接比较。统一比较前需要先转换成同一种格式
（推荐 OCI layout）。

