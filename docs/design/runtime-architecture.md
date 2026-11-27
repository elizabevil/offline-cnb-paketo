# Runtime 架构说明

## 1. 角色

```text
应用源码
  ↓
builder 容器
  ├─ lifecycle
  └─ buildpacks
  ↓
inner runtime
  ↓
s-mart
```

| 角色 | 职责 |
|---|---|
| Outer runtime | 启动 builder、sidecar，执行 import/save |
| Builder | 运行 lifecycle 和 buildpacks |
| sidecar | 为本次构建提供 inner runtime |
| Inner runtime | 接收并保存构建出的镜像 |

## 2. 三种拓扑

### host

```text
outer = inner = 宿主机运行时
```

```text
host-podman
host-docker
```

无 sidecar。

### pind

```text
outer  = 宿主机 Podman
inner  = sidecar Podman
```

适合需要 inner Podman、但不想依赖宿主 Docker 的场景。

### dind

```text
outer  = 宿主机 Podman/Docker
inner  = sidecar Docker
```

适合需要 Docker API 的场景。

## 3. 非 root 推荐

```text
host-podman + builder 1000:1000
pind        + builder 1000:1000
dind        + builder 1000:1000
```

要求：

```text
cache/workspace 可写
sidecar socket 属于非 root
不使用 rootful Docker 的 root-owned socket
```

## 4. 数据流

```text
1. 解析 app_desc
2. 生成 build plan
3. outer runtime 启动 builder
4. builder 连接 inner runtime
5. lifecycle detect/build/analyze/export
6. 导出 layout 或 daemon archive
7. 打包 s-mart
```

## 5. 导出模式

### layout

```text
lifecycle 写 OCI layout
→ bksmartpacker 复制 layout archive
→ s-mart
```

适用：

```text
host-podman
pind
dind（显式 layout）
```

### daemon

```text
lifecycle 写 inner daemon
→ image save
→ docker-archive
→ s-mart
```

适用：

```text
dind
pind daemon
host-docker daemon
```

## 6. workspace

builder 与 sidecar 共享：

```text
/cnb/work
```

sidecar socket：

```text
pind: /cnb/work/run/podman.sock
dind: /cnb/work/run/docker.sock
```

Unix socket 路径必须短于 108 字符。

## 7. 测试入口

```bash
just test-quick
just test-pind
just test-dind
just test-dind-daemon
```

默认非 root builder，详见：

- [rootless-runtime-recommendation.md](rootless-runtime-recommendation.md)
- [runtime-matrix.md](components/bksmartpacker/runtime-matrix.md)
