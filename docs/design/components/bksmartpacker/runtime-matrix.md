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
| E | Podman | Dockerd | managed dind | **已验证** | 非 root builder + 可写 cache；root builder 会触发 lifecycle singleton |
| F | Docker | Dockerd | managed dind | 未测 | 推测可行，需 Docker 外层 + privileged |
| G | Docker | Dockerd | host dind | 未测 | 需要宿主机 Docker daemon |

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

## 4. 当前结论

- host podman、outer podman + inner podman 已验证
- outer podman + inner dockerd 已验证（非 root builder）
- 外层 Docker 场景全部未测
- 不能因为 sidecar 启动成功就认为完整 dind 构建成功

## 5. 后续验证顺序

1. outer Docker + inner Docker（host）
2. outer Docker + inner Podman（managed pind）
3. outer Podman + inner Dockerd 的完整构建
4. 多架构重复上述矩阵
