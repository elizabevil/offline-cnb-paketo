# Runtime 测试矩阵

> 环境：amd64；Podman 5.4.2；Docker 26.1.5 rootful/rootless；qemu-user arm64。

## 1. 术语

| 术语 | 定义 |
|---|---|
| Outer | 宿主机上启动 builder/sidecar 的运行时 |
| Inner | builder 通过 socket 连接的运行时 |
| Builder | builder 容器内进程用户 |
| sidecar | 单次构建启动的辅助容器，提供 inner runtime |
| host-docker | outer/inner 都是宿主 Docker，无 sidecar |
| host-podman | outer/inner 都是宿主 Podman，无 sidecar |
| pind | inner 是 sidecar Podman |
| dind | inner 是 sidecar Docker |

`root` 指宿主机实际 UID。

## 2. 模型

### host

```text
outer = inner = 同一个 daemon
```

Host root 决定 Inner root，不能独立选择。

### managed

```text
outer = 宿主机运行时
inner = sidecar 运行时
```

Managed 的 8 种组合：

| Outer root | Inner root | Builder | Status | Reason |
|---|---|---|---|---|
| root | root | root | pass | — |
| root | root | non-root | not-run | 未执行 |
| root | non-root | root | pass | Docker rootful outer + rootless inner |
| root | non-root | non-root | pass | Docker rootful outer + rootless inner |
| non-root | root | root | blocked | rootless outer 不能启动 rootful inner |
| non-root | root | non-root | blocked | rootless outer 不能启动 rootful inner |
| non-root | non-root | root | pass | — |
| non-root | non-root | non-root | pass | — |

## 3. 测试矩阵

按 Outer root 拆分：

| 文档 | 内容 |
|---|---|
| [runtime-matrix-rootful.md](runtime-matrix-rootful.md) | Outer root = root |
| [runtime-matrix-nonroot.md](runtime-matrix-nonroot.md) | Outer root = non-root |

## 4. 产物

[runtime-artifacts.md](runtime-artifacts.md)

## 5. 未完成

| ID | Platform | 组合 | Status | Reason |
|---|---|---|---|---|
| GAP-01 | arm64 | host-docker rootful + daemon | fail | singleton |
| GAP-02 | arm64 | pind | blocked | qemu robust lock |
| GAP-03 | arm64 | Docker rootless outer + pind | blocked | sidecar 退出 |
| GAP-04 | arm64 | Docker rootless outer + dind daemon | fail | singleton |
| GAP-05 | arm64 | Docker rootful outer + pind | blocked | sidecar 退出 |
| GAP-06 | arm64 | Docker rootful outer + dind daemon | fail | singleton |
| GAP-07 | arm64 | host-docker rootless + daemon | fail | singleton |
| GAP-08 | amd64 | host-docker + builder non-root | fail | singleton |
| GAP-09 | arm64 | host-docker + builder non-root | not-run | 未执行 |

## 6. singleton

```text
root-owned unix socket
lifecycle priv.shouldConnectSock() == true
```

结果：

```text
amd64 host-docker rootful        pass
arm64 host-docker rootful        fail
rootless Docker                  mixed
pind/dind non-root socket        pass
```
