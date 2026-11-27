# Rootless Runtime 推荐方案

> 目标：测试与生产优先使用非 root 运行时和非 root builder。
> 适用范围：bksmartpacker managed/host runtime。
> 概念与拓扑见 [runtime-concepts.md](concepts.md)。

## 1. 结论

推荐顺序：

```text
1. host-podman + non-root builder
2. pind + non-root builder
3. dind + non-root builder
```

不推荐：

```text
rootful Docker + root-owned socket
```

rootful Docker 只用于功能验证，不作为非 root 门禁。

## 2. 方案对比

| 方案 | 需要 Docker | 隔离 | rootless | 推荐场景 |
|---|---|---|---|---|
| host-podman | 否 | 宿主机 | 是 | 最简单，默认 |
| pind | 否 | sidecar | 是 | 需要 inner Podman |
| dind | 是 | sidecar | 是 | 需要 Docker API |
| rootless Docker host | 是 | 宿主机 | 是 | 必须用宿主 Docker；builder non-root 当前受限 |

## 3. 推荐命令

### 3.1 host-podman

```bash
RUNTIME=host-podman \
BKS_CONTAINER_CLI=podman \
BKS_BUILDER_USER=1000:1000 \
BKS_BUILDER_USERNS=keep-id \
CNB_CACHE_DIR=/path/to/cache \
bksmartpacker \
  --src /path/to/app \
  --builder-image offline-builder-build-bk:amd64 \
  --dest s-mart.tar.gz
```

要求：

```text
Podman 以非 root 运行
cache/workspace 对 builder 用户可写
```

### 3.2 pind

```bash
RUNTIME=pind \
BKS_RUNTIME_DRIVER=managed \
CNB_RUNTIME_IMAGE=offline-runtime-pind-bk:amd64 \
BKS_CONTAINER_CLI=podman \
BKS_BUILDER_USER=1000:1000 \
BKS_BUILDER_USERNS=keep-id \
CNB_RUNTIME_WORKSPACE=/path/to/workspace \
CNB_CACHE_DIR=/path/to/cache \
bksmartpacker \
  --src /path/to/app \
  --builder-image offline-builder-build-bk:amd64 \
  --dest s-mart.tar.gz
```

### 3.3 dind

```bash
RUNTIME=dind \
BKS_RUNTIME_DRIVER=managed \
CNB_RUNTIME_IMAGE=offline-runtime-dind-bk:amd64 \
BKS_CONTAINER_CLI=podman \
BKS_INNER_CONTAINER_CLI=docker \
BKS_BUILDER_USER=1000:1000 \
BKS_BUILDER_USERNS=keep-id \
CNB_RUNTIME_WORKSPACE=/path/to/workspace \
CNB_CACHE_DIR=/path/to/cache \
BKS_EXPORT_MODE=layout \
bksmartpacker \
  --src /path/to/app \
  --builder-image offline-builder-build-bk:amd64 \
  --dest s-mart.tar.gz
```

需要 daemon 导出时：

```bash
BKS_EXPORT_MODE=daemon
```

### 3.4 rootless Docker host（受限）

```bash
DOCKER_HOST=unix:///run/user/1000/docker.sock \
BKS_DOCKER_SOCK=/run/user/1000/docker.sock \
RUNTIME=host-docker \
BKS_CONTAINER_CLI=docker \
BKS_BUILDER_USER=1000:1000 \
CNB_CACHE_DIR=/path/to/cache \
bksmartpacker \
  --src /path/to/app \
  --builder-image offline-builder-build-bk:amd64 \
  --dest s-mart.tar.gz
```

## 4. 判断规则

```text
需要 Podman inner
  → pind

需要 Docker inner
  → dind

只需要 host runtime
  → host-podman

必须用宿主 Docker
  → rootless Docker host（builder non-root 当前 fail）
```

## 5. 不推荐路径

```text
host-docker + rootful Docker
```

问题：

```text
/var/run/docker.sock root-owned
lifecycle shouldConnectSock() == true
→ singleton connection already in use
```

已验证：

```text
host-docker rootful + non-root builder  fail singleton
host-docker rootless + non-root builder fail singleton
```

因此不能把 rootful Docker 作为非 root 方案。

## 6. 硬性前提

```text
builder 用户 1000:1000
builder cache 可写
workspace 可写
sidecar socket 属主为非 root
```

rootless Docker 额外要求：

```text
rootlesskit
slirp4netns 或 pasta
subuid/subgid
/run/user/1000/docker.sock
```

## 7. 当前验证状态

| 方案 | 状态 |
|---|---|
| host-podman + non-root builder | pass |
| pind + non-root builder | pass amd64；arm64 qemu 限制 |
| dind + non-root builder | pass amd64/arm64 |
| rootless Docker + non-root builder | fail singleton |
| rootful Docker + non-root builder | fail singleton |

## 8. 推荐

默认：

```text
host-podman + non-root builder
```

需要 sidecar：

```text
pind + non-root builder（Podman）
dind + non-root builder（Docker）
```

需要宿主 Docker：

```text
rootless Docker
```

不要使用 rootful Docker 作为非 root 路径。
