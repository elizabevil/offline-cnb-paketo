# 真实 run image、pind/dind 与 RUNTIME_WORKSPACE

> 状态：方案分析，未实施。

## 1. 当前为什么需要 fake runner

当前 `ContainerExecutor` 在每次构建前：

1. 创建只含一个空文件的 rootfs
2. `podman/docker import` 成单层空镜像
3. 导出为 `run-image.tar`
4. 把 `CNB_RUN_IMAGE` 和 `CNB_RUN_IMAGE_ARCHIVE` 指向这个 fake runner

原因：

- CNB lifecycle exporter 需要一个合法 run image 作为镜像基础
- 当前本地构建只负责产出应用层，不需要真实 OS 运行层
- 使用真实 run image 会让本地构建依赖变大，并可能和平台 base image 重复

因此 fake runner 是“本地 exporter 的占位基础镜像”，不是最终运行镜像。

## 2. 特殊项目自定义运行层

特殊项目可能需要：

- 非默认 libc / OpenSSL / 系统库
- 自定义 CA、时区、字体
- 特殊解释器或系统工具
- 与平台默认 base image 不同的运行层

需要新增的能力：

| 能力 | 说明 |
|---|---|
| `--run-image` | 使用本地/远程真实 run image 引用 |
| `--run-image-tar` | 使用 docker-archive / OCI archive |
| architecture 校验 | run image 架构必须与目标一致 |
| libc/OS 校验 | 避免 bionic/trixie 混用 |
| platform base 映射 | artifact 的 `base_image_id` 仍需与平台配置对应 |
| 包体积策略 | 真实运行层会显著增大 s-mart |
| 离线策略 | 允许本地 tar，避免构建时拉取 |

注意：

```
artifact.base_image_id 只声明平台 base image key，
不会自动把自定义 run image 变成平台运行层。
```

要真正使用自定义运行层，需要同时满足：

1. 构建时 exporter 使用该 run image
2. 发布产物包含对应层，或平台 base image 配置指向同一运行层
3. 平台部署时不再用默认 base 覆盖

## 3. 当前 pind/dind 是什么

当前 `pkg/container` 只有配置预设：

```text
Backend:      docker | podman
Environment:  host | pind | dind
```

| Environment | 当前行为 |
|---|---|
| host | 使用宿主机运行时 socket |
| pind | 选择 podman 后端 + `/run/podman/podman.sock` + layout |
| dind | 选择 docker 后端 + daemon 导出 |

当前**没有**：

- 启动/停止嵌套 Podman
- 启动/停止嵌套 Docker daemon
- 管理嵌套运行时存储
- 管理 runtime socket 生命周期
- 管理 cgroup / user namespace / 权限
- 隔离 runtime workspace

所以当前 `pind/dind` 只是“连接参数预设”。

## 4. 完整 pind/dind 需要什么

### 4.1 运行时进程

- 在构建环境内启动 `podman system service` 或 `dockerd`
- 等待 socket ready
- 设置 rootful/rootless
- 处理 storage driver：
  - overlay
  - fuse-overlayfs
  - vfs
- 处理 cgroup v2
- 退出时清理进程

### 4.2 Socket

- 生成 runtime socket
- 设置权限和所属用户
- 挂载到 builder 容器
- 统一 `DOCKER_HOST` / `CONTAINER_HOST`
- 处理 socket 已存在/残留

### 4.3 Workspace

- runtime graphroot / runroot
- 容器 storage
- socket
- 临时层
- 缓存
- 构建产物

### 4.4 权限

- rootless user namespace
- rootful 需要 `--privileged` 或精确 cap
- `/dev/fuse`
- SELinux label
- seccomp / apparmor
- 文件属主

### 4.5 网络

- host/桥接网络
- DNS
- registry / 内网镜像源
- 代理
- 端口冲突

### 4.6 镜像

- 把 run image 导入嵌套运行时
- 缓存 build image
- 导出 image tar
- registry auth
- 多架构

### 4.7 生命周期与容错

- 启动失败回退
- 超时
- 并发构建隔离
- 残留容器/镜像清理
- 磁盘配额

## 5. 当前与完整 pind/dind 的差异

| 项 | 当前 | 完整 pind/dind |
|---|---|---|
| 运行时进程 | 使用宿主机已有进程 | 在构建环境内启动 |
| socket | 挂载宿主机 socket | 自建 socket |
| storage | 宿主机默认存储 | 独立 workspace |
| 隔离 | 依赖宿主机 | 构建级隔离 |
| 权限 | 由宿主决定 | 需要 namespace/cgroup/cap 管理 |
| 清理 | 无 | 必须清理 |
| 适用 | 本机已装 podman/docker | CI/受限环境/需要隔离 |
| 实现量 | 配置解析 | 运行时编排 |

## 6. RUNTIME_WORKSPACE 怎么理解

官方默认：

```text
RUNTIME_WORKSPACE=/podman/smart-app
```

官方参数只承诺它是一个**构建运行时的工作目录**；具体子目录由运行时实现决定。
在本地方案中建议把它定义为：

```text
RUNTIME_WORKSPACE/
├── src/           # 解压后的源码（可选）
├── platform/      # platform env
├── layers/        # 构建层输出
├── run/           # 运行时 socket / pid / 临时状态
├── storage/       # 嵌套运行时 storage（如需要）
└── tmp/           # 临时文件
```

它不是最终 s-mart 产物目录；最终产物仍由 `--dest` 指定。

### 6.1 逻辑路径对应表

| 逻辑路径 | 内容 | 当前 host 实现 | managed pind/dind | 生命周期 |
|---|---|---|---|---|
| `<ws>/src` | 解压源码 / 源码快照 | 临时 `source.tgz` | 建议挂载到 `/tmp/source.tgz` | 单次构建 |
| `<ws>/platform/env` | buildpack 变量 | 临时 `platform/env` | 挂载到 `/platform` | 单次构建 |
| `<ws>/layers` | lifecycle 层输出 | `<tmp>/layers` | 挂载到 `/cnb-layer` | 单次构建 |
| `<ws>/run` | socket / pid / runtime 临时状态 | 宿主机 socket | 挂载到 `/var/run/docker.sock` | pind/dind 进程生命周期 |
| `<ws>/storage` | 镜像/容器存储 | 宿主机默认存储 | podman `--root` / `--runroot`；dockerd `--data-root` | 构建环境生命周期 |
| `<ws>/tmp` | 临时文件 | 系统 temp | 独立 temp | 构建环境生命周期 |
| `/layers/cache` | 构建缓存 | 项目挂载 | 仍独立挂载，不强制进 workspace | 跨构建 |
| `--dest` | 最终 s-mart | 项目输出路径 | 不变 | 构建结束 |

约束：

- `<ws>` 不是最终产物目录
- `<ws>` 必须对构建用户可写
- host 模式不要求 `<ws>`
- managed 模式要求 `<ws>` 可挂载、可清理
- 不要让 `<ws>` 与 `/layers/cache` 混用



如果要支持完整 pind/dind，需要把 runtime storage/runroot 显式落到 `RUNTIME_WORKSPACE`，并保证：

- 目录可写
- 属主与运行用户一致
- 可挂载进构建容器
- 构建结束后可清理
- 不污染宿主机默认 podman/docker 存储

## 7. 与当前 host 方案的兼容策略

### 7.1 先确认运行层的决定权

当前平台部署时：

```text
artifact.json
  runtime.base_image_id = "default"
        ↓
平台 SMART_CNB_IMAGE_CONF["default"]
        ↓
实际 run image
```

本地 image tar 只提供应用层，fake runner 只是 exporter 的占位基础。
因此：

- `--run-image` 只影响构建时 OS 库和 exporter 输入
- 不会自动改变平台运行时
- 自定义运行层必须同时有平台 base image key

### 7.2 增加 Driver 维度，不破坏现有语义

当前：

```text
Backend:      docker | podman
Environment:  host | pind | dind
```

建议只增加一个维度：

```text
Driver:       host | managed
```

| Driver | 行为 |
|---|---|
| host | 使用已有宿主机运行时 socket，当前默认 |
| managed | 在构建环境内启动/停止运行时 |

兼容规则：

```text
host + pind  = 当前外部 podman socket 预设
managed + pind = 完整 podman-in-docker
host + dind  = 当前外部 docker daemon
managed + dind = 完整 docker-in-docker
```

这样现有 `RUNTIME=pind` 不会突然变成“启动 Podman”，默认行为保持不变。

建议环境变量：

```text
BKS_RUNTIME_DRIVER=host|managed   # 默认 host
CNB_RUNTIME_WORKSPACE=<path>      # managed 时使用
```

### 7.3 最小化实施路径

#### 阶段 0：只留接口

- `RuntimeDriver` 接口
- `HostDriver` 包装当前 host 行为
- 不改变任何默认路径

#### 阶段 1：workspace 可选

- `CNB_RUNTIME_WORKSPACE` 存在时，把 build/platform/layers/run 放进去
- 不存在时继续用临时目录
- 不启动任何 daemon

#### 阶段 2：外部 socket 模式的 pind

- 仍使用已有 podman socket
- 增加 workspace 和显式状态检查
- 这是当前能力的增强，不是完整 pind

#### 阶段 3：managed pind

- 启动 `podman system service`
- 使用 runtime workspace 的 storage/runroot
- 生成 socket，等待 ready
- 构建结束清理
- 需要 rootful/privileged 或 rootless namespace

#### 阶段 4：managed dind

- 启动 dockerd
- 独立 data-root/exec-root
- daemon 导出
- 清理

### 7.4 不建议的兼容方式

| 方式 | 问题 |
|---|---|
| 直接把当前 `pind` 改成 managed | 破坏现有 host 行为 |
| managed 失败静默回退 host | 难以发现隔离和权限问题 |
| 所有模式都强制 RUNTIME_WORKSPACE | host 模式无必要，增加迁移成本 |
| 先做 dind | 权限和 daemon 生命周期更复杂 |

### 7.5 最小可用结论

如果暂时不想承担完整 pind/dind：

1. 保持 host 默认
2. 增加自定义 run image（opt-in）
3. 增加可选 `RUNTIME_WORKSPACE`
4. 把 `pind/dind` 明确定义为 host 模式下的 socket 预设

完整 managed pind/dind 作为独立 driver，后续增量实现。

## 8. 构建镜像需要什么调整

从宿主机看，先判断运行时在哪里：

```text
宿主机已有 podman/docker
  → builder 不需要 daemon，host 模式

宿主机需要隔离 / 没有可用运行时
  → managed pind/dind
  → 运行时放 sidecar 容器或单容器变体
```

builder 镜像是否包含 daemon，不取决于“要不要 pind”，而取决于部署形态。

当前 `builder-build.dockerfile` 主要是：

```text
lifecycle
buildpacks
cnb
fnm / uv
jq / yq / pack
entrypoint
```

它不是“把 docker/podman 二进制丢进去”就能支持 pind/dind 的。

### 8.1 host 模式

保持现状即可：

- builder 镜像不需要 docker/podman daemon
- 不需要 dockerd / podman service
- 只需要 lifecycle、buildpacks、构建工具
- 宿主机 socket 通过 `--mount` 传入

当前 Dockerfile 中空的 Docker CLI 注释应删除或明确标注“host 模式不需要”。

### 8.2 自定义 run image

builder 镜像基本不需要新增二进制，主要改运行时参数：

| 调整 | 原因 |
|---|---|
| 传入 `CNB_RUN_IMAGE` / `CNB_RUN_IMAGE_ARCHIVE` | lifecycle 需要真实 run image |
| 挂载 run image tar | 离线、避免构建时拉取 |
| 校验 architecture | 镜像架构必须一致 |
| 校验 `CNB_STACK_ID` | build image / run image stack 必须兼容 |
| 对齐平台 `base_image_id` | artifact 只声明 key，平台实际选 base |

注意：

```text
builder 镜像的 OS 与 run image 的 OS 不是同一个概念
```

builder 镜像用于 detect/build，run image 用于 launch。
如果 buildpack 产出的是针对特定 libc 编译的二进制，build 与 run 的 stack 必须匹配。

### 8.3 managed pind/dind

完整 pind/dind 需要的不只是二进制：

| 组件 | pind | dind | 说明 |
|---|---|---|---|
| podman | 需要 | 不需要 | 含 conmon/crun/storage 依赖 |
| dockerd | 不需要 | 需要 | 含 containerd/runc/iptables |
| slirp4netns | rootless 需要 | 不需要 | 用户态网络 |
| fuse-overlayfs | rootless 需要 | 不需要 | storage driver |
| newuidmap/newgidmap | rootless 需要 | 不需要 | user namespace |
| cgroup 工具 | 需要 | 需要 | 资源限制 |
| iptables/nftables | 视网络 | 需要 | 端口/NAT |
| CA 证书 | 需要 | 需要 | registry TLS |
| socket 启动脚本 | 需要 | 需要 | `podman system service` / `dockerd` |
| 清理逻辑 | 需要 | 需要 | 容器/镜像/workspace |

还要有容器运行参数：

- `--privileged` 或精确 capabilities
- `/dev/fuse`
- cgroup namespace
- SELinux label
- 独立 `RUNTIME_WORKSPACE`
- socket 挂载
- 网络模式

### 8.4 默认 builder 不包含 daemon

宿主机的 host 模式与 managed 模式应并存：

- 默认 builder 镜像只做构建
- managed 作为独立 runtime driver
- 只有单容器约束才引入 builder-pind/dind 变体

有两种实现：

#### 方案 A：runtime sidecar（推荐）

```text
bksmartpacker
  ├─ 启动 runtime 容器（podman/dind）
  ├─ 等待 socket
  ├─ builder 容器挂载该 socket
  └─ 构建结束清理 sidecar
```

builder 镜像保持不变，只增加：

- socket 挂载
- workspace 挂载
- 运行时健康检查

#### 方案 B：nested runtime image

```text
builder-pind 镜像
  = builder-build + podman + 依赖 + entrypoint
```

适合必须单容器交付的场景，但：

- 镜像明显变大
- 需要 privileged
- 启动/清理复杂
- 与 host 模式差异大

### 8.5 调整结论

```text
host 模式             → builder 镜像不变
自定义 run image      → 只改参数/挂载/stack 校验
managed pind/dind     → 独立 runtime driver；不要默认塞进 builder
```

如果只支持 host + 自定义 run image，当前 builder 镜像不需要增加 docker/podman daemon。

## 9. managed pind/dind 的部署形态

### 9.1 从宿主机看

managed 模式不可能凭空启动运行时，宿主机仍然需要：

```text
一个外层容器运行时（podman / docker）
或 Kubernetes / 其他编排器
```

区别只在于：运行时是放在宿主机，还是放进构建环境。

```text
host 模式：
  bksmartpacker → 宿主机 socket → builder 容器

managed 模式：
  bksmartpacker → 启动 runtime 容器 / 单容器
                → runtime 内部 socket
                → builder 使用该 socket
```

### 9.2 形态 A：整合镜像包（runtime sidecar）

这是多镜像离线包，不是单容器。

```text
project/offline-paketo-bk/
├── docker/
│   ├── builder-base.dockerfile
│   ├── builder-build.dockerfile
│   ├── runtime-pind.dockerfile
│   ├── runtime-dind.dockerfile
│   └── runtime/
│       ├── entrypoint-pind.sh
│       ├── entrypoint-dind.sh
│       └── storage.conf
├── pipeline.toml
└── justfile

dist/offline-builder-bundle/
├── bin/
│   └── bksmartpacker
├── images/
│   ├── builder-build-<arch>.tar
│   ├── runtime-pind-<arch>.tar
│   └── runtime-dind-<arch>.tar
└── manifest.json
```

运行关系：

```text
宿主机 podman/docker
  ├─ runtime-pind 容器
  │    └─ /run/podman/podman.sock
  └─ builder 容器
       └─ 挂载 runtime socket + RUNTIME_WORKSPACE
```

特点：

- builder 镜像保持轻量
- runtime 镜像独立升级
- 需要两个容器和 socket/network 协调
- 宿主机仍需要外层运行时

需要变更：

| 位置 | 变更 |
|---|---|
| `docker/runtime-*.dockerfile` | 新增 podman/dind 运行时镜像 |
| `pipeline.toml` | 新增 runtime-pind/dind stages |
| `project/assets.toml` | 登记 podman/docker 运行时物料 |
| `project/offline-paketo-bk/justfile` | 新增 bundle-images / load 配方 |
| `pkg/container` | 增加 Driver、Workspace、RuntimeEndpoint |
| `executors` | 支持连接外部 runtime socket，不改变 builder 镜像 |
| `bundle/manifest.json` | 记录 builder 与 runtime 镜像关系 |

### 9.2.1 关键：socket 和 workspace 必须同源

`runtime-pind` 不只是“提供 socket”。builder 里的 lifecycle 会通过该 socket
让 runtime 启动构建容器，而构建容器挂载的路径来自 lifecycle 参数。

如果 socket 在 sidecar，但 source/platform/layers 只在 builder 里，runtime 侧看不到这些路径，
构建容器会挂载空目录或失败。

因此 sidecar 必须共享同一个宿主机 workspace：

```text
宿主机共享目录 /var/lib/bksmart/run-<id>/
├── src/source.tgz
├── platform/env/
├── layers/
├── run/podman.sock
└── storage/

runtime-pind:
  挂载 same dir → /runtime-workspace
  socket 位于 /runtime-workspace/run/podman.sock
  podman storage 位于 /runtime-workspace/storage

builder:
  挂载 same dir → /runtime-workspace
  DOCKER_HOST=unix:///runtime-workspace/run/podman.sock
  lifecycle 参数使用 /runtime-workspace/... 路径
```

结论：

```text
runtime-pind 提供：podman service + socket + storage
builder 提供：lifecycle + source + platform
两者共享：RUNTIME_WORKSPACE（同一宿主机路径，相同容器内路径）
```

只挂 socket 不够，必须同时解决文件路径一致性。

### 9.3 形态 B：单容器独立镜像变体

把 runtime 和 builder 合进一个镜像：

```text
project/offline-paketo-bk/docker/
├── builder-base.dockerfile
├── builder-build.dockerfile
├── builder-pind.dockerfile
├── builder-dind.dockerfile
├── entrypoint-pind.sh
└── entrypoint-dind.sh
```

镜像：

```text
offline-builder-pind-bk:<arch>
offline-builder-dind-bk:<arch>
```

运行关系：

```text
宿主机外层运行时
  └─ builder-pind 容器（privileged）
       ├─ 启动 podman service
       ├─ 同容器内 lifecycle 构建
       └─ 退出清理
```

特点：

- 单容器交付
- 镜像明显变大
- 需要 privileged / cap / cgroup / fuse
- entrypoint 要同时管理 runtime 与 lifecycle
- 宿主机只负责启动一个容器

需要变更：

| 位置 | 变更 |
|---|---|
| `builder-pind.dockerfile` | `FROM builder-build` + podman 依赖 + entrypoint |
| `builder-dind.dockerfile` | `FROM builder-build` + dockerd 依赖 + entrypoint |
| `pipeline.toml` | 新增 builder-pind/dind stages |
| `project/offline-paketo-bk/justfile` | 构建 variant、选择 builder 镜像 |
| `pkg/container` | Driver=managed；确定 workspace/socket |
| `executors` | 启停单容器、传 privileged/cap、健康检查 |
| `lifecycle-entrypoint.sh` | 或合并到 runtime entrypoint，先启动 service 再构建 |

### 9.4 两种形态对比

| 项 | 整合镜像包（sidecar） | 单容器独立变体 |
|---|---|---|
| builder 镜像 | 轻量、不变 | 变重 |
| runtime 镜像 | 独立 | 合并 |
| 宿主机外层运行时 | 需要 | 需要 |
| 权限 | runtime 容器需要 | 单容器需要 |
| 启动协调 | 两个容器 | 一个容器 |
| 升级 | 分开 | 一起 |
| 隔离 | 更好 | 较弱 |
| 实现量 | 中 | 高 |
| 推荐 | **优先** | 有单容器约束时 |

### 9.5 推荐路径

```text
先做 sidecar 整合镜像包
  → builder 不变
  → runtime 镜像单独维护
  → Driver/Workspace 接口先落地

如果部署环境强制单容器
  → 再做 builder-pind / builder-dind 变体
  → 不把变体逻辑混进默认 builder
```

## 10. 建议实施顺序

1. 真实 run image（opt-in）
   - `--run-image` / `--run-image-tar`
   - 架构/libc 校验
   - 与平台 base image 的关系写清楚
2. 显式 runtime workspace
   - 仅 host 模式先支持目录配置
   - 不改变默认行为
3. pind 生命周期
   - 启动 podman service
   - socket/workspace/清理
4. dind 生命周期
   - dockerd、storage、权限
5. 平台/CI 集成与测试

## 11. 结论

```text
fake runner
  = 本地 exporter 的占位基础镜像

真实 run image
  = 需要和构建、产物、平台 base 三处同时对齐

当前 pind/dind
  = 配置预设，不是完整嵌套运行时

完整 pind/dind
  = 运行时进程 + socket + workspace + 权限 + 网络 + 生命周期
```

`RUNTIME_WORKSPACE` 是完整 pind/dind 的**运行时状态根目录**，不是应用源码目录。


## 12. 与 offline-paketo / offline-paketo-bk 的关系

### 12.1 两个项目的定位不同

| 项目 | 产物 | 是否包含执行器 |
|---|---|---|
| `project/offline-paketo` | `offline-builder:<arch>` 构建镜像 | 否 |
| `project/offline-paketo-bk` | builder + bksmartpacker + s-mart 协议适配 | 是 |

`offline-paketo` 是**builder 镜像工厂**，只负责离线构建 lifecycle/buildpacks/runtime tools。
它不包含 bksmartpacker，也不处理 application package、artifact.json、进程入口。

因此：

```text
offline-paketo 本身不能直接支持 pind/dind 运行
```

pind/dind 是**执行器运行时模式**，不是 builder 镜像格式，也不是 s-mart 包格式。

### 12.2 可扩展方式

#### 方式 A：sidecar，在 offline-paketo 增加 runtime 镜像

```text
project/offline-paketo/
├── docker/
│   ├── base.dockerfile
│   ├── builder.dockerfile
│   ├── runtime-pind.dockerfile
│   ├── runtime-dind.dockerfile
│   └── runtime/
│       ├── entrypoint-pind.sh
│       └── entrypoint-dind.sh
└── pipeline.toml
```

`offline-paketo` 继续产出 builder，同时增加 runtime 镜像。
`offline-paketo-bk` 的 bksmartpacker 通过 socket/workspace 连接 runtime。

优点：builder 不变；runtime 可被其他消费者复用。

#### 方式 B：单容器，在 offline-paketo 增加 builder 变体

```text
project/offline-paketo/
├── docker/
│   ├── base.dockerfile
│   ├── builder.dockerfile
│   ├── builder-pind.dockerfile
│   └── builder-dind.dockerfile
└── pipeline.toml
```

`builder-pind` / `builder-dind` 都 `FROM` 现有 builder。
这是单容器方案，镜像更大、需要 privileged。

### 12.3 与 offline-paketo-bk 的关系

`offline-paketo-bk` 是 **BK 协议适配层**：

```text
offline-paketo       → 通用 builder 镜像
offline-paketo-bk    → builder + bksmartpacker + s-mart/artifact/进程
```

因此：

| 能力 | 放哪里 |
|---|---|
| lifecycle/buildpacks/runtime tools | `offline-paketo` |
| podman/dind runtime 镜像 | `offline-paketo` 或独立 runtime 项目 |
| bksmartpacker 执行器 | `offline-paketo-bk` |
| s-mart / artifact / Procfile | `offline-paketo-bk` |
| managed runtime driver | bksmartpacker，放在共享 Go 包更合适 |

### 12.4 推荐

```text
offline-paketo       保持 builder 工厂，不塞执行器
offline-paketo-bk    继续负责 BK 执行器与协议
runtime images       共享，不要在两个项目各复制一份
```

最小路径：

1. `offline-paketo` 只新增 runtime 镜像（或独立 runtime 项目）
2. `offline-paketo-bk` 的 bksmartpacker 增加 Driver/Workspace
3. 先在 BK 项目跑通 sidecar
4. 再决定是否把单容器变体放进 offline-paketo


## 13. host 模式兼容契约

host 模式必须长期保持一等支持，managed 只能是显式 opt-in。

### 13.1 默认行为不变

| 配置 | 默认 | 行为 |
|---|---|---|
| `BKS_RUNTIME_DRIVER` | 未设置 = host | 使用宿主机已有运行时 |
| `CNB_RUNTIME_WORKSPACE` | 未设置 | 继续使用临时目录 |
| `BKS_CONTAINER_CLI=podman` | 现有用法 | 宿主机 podman |
| `RUNTIME=pind` | 现有用法 | podman rootful socket 预设，不启动 daemon |
| builder 镜像 | `offline-builder-build-bk:<arch>` | 不包含 daemon，不改变 |

验收标准：

```text
不设置任何新变量
→ just smart 与当前完全一致
→ socket 路径不变
→ builder 镜像不变
→ artifact.json / s-mart 不变
```

### 13.2 managed 只能显式开启

```text
BKS_RUNTIME_DRIVER=managed
```

未显式设置时：

- 不启动 sidecar
- 不启动 daemon
- 不要求 RUNTIME_WORKSPACE
- 不增加 privileged
- 不改变 socket 挂载

managed 启动失败时：

```text
直接报错，不静默回退 host
```

避免用户以为在 managed 环境，实际回退到宿主机。

### 13.3 兼容矩阵

| 模式 | 宿主机运行时 | Driver | Workspace | builder daemon | 默认 |
|---|---|---|---|---|---|
| host podman | 需要 | host | 不需要 | 无 | 是 |
| host docker | 需要 | host | 不需要 | 无 | 是 |
| 外部 pind socket | 需要 | host | 可选 | 无 | 兼容 |
| 外部 dind socket | 需要 | host | 可选 | 无 | 兼容 |
| sidecar pind | 需要外层运行时 | managed | 需要 | 无 | opt-in |
| sidecar dind | 需要外层运行时 | managed | 需要 | 无 | opt-in |
| 单容器 pind | 需要外层运行时 | managed | 需要 | 有 | opt-in |
| 单容器 dind | 需要外层运行时 | managed | 需要 | 有 | opt-in |

### 13.4 代码层兼容点

| 层 | 要求 |
|---|---|
| `pkg/container` | Driver 默认为零值 host；Host 分支保持现有 socket/export |
| `executors` | host 分支不读 workspace；参数与当前一致 |
| CLI | 不增加必填参数；新配置全部可选 |
| `justfile` | 现有 `BKS_CONTAINER_CLI=podman` 继续可用 |
| builder 镜像 | 默认镜像不带 daemon |
| artifact | 不增加 host 模式必填字段 |

### 13.5 回归测试

必须保留：

1. `BKS_CONTAINER_CLI=podman` 构建 SOPS
2. 不设置 `BKS_RUNTIME_DRIVER` 时行为不变
3. `RUNTIME=pind` 仍只作为 socket 预设
4. 包结构、artifact、进程入口不变
5. builder 镜像 tag 不变

### 13.6 演进原则

```text
host 是基座
managed 是插件
sidecar 是默认 managed 形态
单容器是受约束变体
```

任何 managed 改动不得改变 host 分支的默认参数、默认镜像、默认 socket 和默认产物。


## 14. BK 先行、再回流 offline-paketo 的改造路线

### 14.1 结论

合理，但前提是：

```text
BK 只做验证场
通用 runtime 镜像/驱动最终要抽出
不能把 BK 专属逻辑复制进 offline-paketo
```

### 14.2 为什么先在 BK 做

- BK 有完整执行器 `bksmartpacker`
- BK 有 s-mart / artifact / Procfile 端到端验证
- BK 当前已有 host/podman 运行路径
- offline-paketo 只有 builder 镜像，没有执行器，无法单独验证 managed 运行

因此：

```text
先在 BK 验证“managed runtime + builder + 产物”
再把通用部分回流 offline-paketo
```

### 14.3 阶段划分

#### 阶段 0：基线冻结

- host 模式构建 SOPS
- 记录包大小、artifact、进程入口
- 作为后续回归基线

#### 阶段 1：BK 增加 managed sidecar

新增：

```text
project/offline-paketo-bk/
├── docker/
│   ├── runtime-pind.dockerfile
│   ├── runtime-dind.dockerfile
│   └── runtime/
│       ├── entrypoint-pind.sh
│       ├── entrypoint-dind.sh
│       └── storage.conf
├── pipeline.toml          # runtime stages
└── justfile               # bundle/runtime 配方
```

代码：

```text
pkg/container
  + Driver: host | managed
  + Workspace
  + RuntimeEndpoint

pkg/builder/executors
  + sidecar 启动/等待/清理
  + socket + workspace 挂载
```

约束：

- 默认 Driver=host
- builder-build 镜像不改
- host 分支参数与当前一致

#### 阶段 2：BK 验证 managed pind

验证项：

- runtime-pind 容器启动
- socket ready
- builder 与 runtime 共享 workspace
- lifecycle 构建容器路径一致
- 产物仍为单个 image tar + artifact
- 清理 runtime 容器

#### 阶段 3：抽出通用部分

把以下内容从 BK 抽到共享位置：

```text
runtime-pind.dockerfile
runtime-dind.dockerfile
runtime entrypoint
storage.conf
runtime assets
```

建议位置：

```text
project/offline-paketo/docker/runtime/
```

或者独立：

```text
project/offline-runtime/
```

BK 不再保留副本，只引用镜像。

#### 阶段 4：回流 offline-paketo

offline-paketo 只增加：

```text
runtime-pind / runtime-dind 构建 stage
runtime 镜像 tag
runtime 健康检查
```

不增加：

```text
bksmartpacker
s-mart
artifact
```

因为 offline-paketo 仍只负责 builder/runtime 镜像。

#### 阶段 5：单容器变体

如果 sidecar 验证通过，再考虑：

```text
offline-paketo/docker/builder-pind.dockerfile
offline-paketo/docker/builder-dind.dockerfile
```

BK 只负责选择镜像和传入 managed Driver。

### 14.4 目录变更

#### BK 阶段

```text
project/offline-paketo-bk/
├── docker/
│   ├── builder-base.dockerfile
│   ├── builder-build.dockerfile
│   ├── runtime-pind.dockerfile      # 临时，验证后迁出
│   ├── runtime-dind.dockerfile
│   └── runtime/
├── pipeline.toml
└── justfile
```

#### 回流后

```text
project/offline-paketo/
├── docker/
│   ├── base.dockerfile
│   ├── builder.dockerfile
│   └── runtime/
│       ├── pind.dockerfile
│       ├── dind.dockerfile
│       └── entrypoint-*.sh
└── pipeline.toml

project/offline-paketo-bk/
├── docker/
│   ├── builder-base.dockerfile
│   └── builder-build.dockerfile
└── 只消费 runtime 镜像
```

### 14.5 代码归属

| 能力 | 归属 |
|---|---|
| runtime 镜像 | offline-paketo |
| runtime 镜像构建 stage | offline-paketo |
| Driver/Workspace | bksmartpacker 共享 Go 包 |
| sidecar 启停 | bksmartpacker |
| s-mart/artifact | offline-paketo-bk |
| host 模式 | 不变 |

### 14.6 验收标准

必须同时满足：

1. host 模式 SOPS 产物不变
2. managed pind 产物结构不变
3. builder-build 镜像默认不变
4. runtime 镜像可独立构建
5. offline-paketo 不引入 bksmartpacker
6. BK 不保留 runtime Dockerfile 副本

### 14.7 不推荐的做法

| 做法 | 问题 |
|---|---|
| BK 完成后直接复制 Dockerfile 到 offline-paketo | 双份维护漂移 |
| 把 Driver 放进 offline-paketo | offline-paketo 没有执行器 |
| 把 bksmartpacker 放进 offline-paketo | 破坏项目边界 |
| 先做单容器再做 sidecar | 权限/entrypoint 复杂度更高 |


## 15. 目录与挂载简化方案

### 15.1 当前问题

当前 executor 的挂载点分散：

```text
<tmp>/source.tgz          → /tmp/source.tgz
<tmp>/platform            → /platform
<workspace>/layers        → /cnb-layer
<workspace>/run-image.tar → /cnb-run/run-image.tar
宿主机 socket             → /var/run/docker.sock
```

问题：

- 宿主机路径和容器路径没有统一关系
- 多个临时目录分别挂载
- sidecar 模式下 builder 与 runtime 需要看到同一路径，当前结构很难保证
- 构建参数里散落多个绝对路径
- 清理逻辑分散

### 15.2 目标：一个 workspace 根

固定容器内根路径：

```text
/cnb/work
```

宿主机对应：

```text
RUNTIME_WORKSPACE 或 executor 临时目录
```

两者通过一次 bind mount 映射：

```text
<host-workspace>  →  /cnb/work
```

内部布局：

```text
/cnb/work/
├── source/
│   └── source.tgz
├── platform/
│   └── env/
├── layers/
│   └── <group>.tar
├── run/
│   ├── run-image.tar
│   └── podman.sock          # managed 模式
├── runtime/
│   ├── storage/             # managed 模式
│   └── runroot/
└── tmp/
```

### 15.3 挂载表

| 宿主机路径 | 容器路径 | 模式 | 出现条件 |
|---|---|---|---|
| `<workspace>` | `/cnb/work` | rw | 始终 |
| `<workspace>/platform` | `/platform` | ro/rw | lifecycle 固定路径 |
| 宿主机 socket | `/var/run/docker.sock` | rw | host 模式 |
| `<workspace>/run/podman.sock` | 不单独挂 | — | managed 模式，随根挂载可见 |
| `<cache>` | `/layers/cache` | rw | 配置了缓存 |
| `<runtimes>` | `/cnb/runtimes` | ro | 配置了 runtimes |
| 用户额外挂载 | 原样 | 原样 | `BKS_EXTRA_MOUNTS` |

说明：

- lifecycle 的 platform 目录固定是 `/platform`，所以
  `<workspace>/platform` 仍要单独绑定到 `/platform`，或由 entrypoint 建立符号链接。
- 除 platform 和 host socket 外，其余构建输入/输出都在 `/cnb/work` 下。

managed 模式：

- socket 位于 `/cnb/work/run/podman.sock`
- 不再单独挂 socket
- runtime 的 storage/runroot 位于 `/cnb/work/runtime`
- builder 与 runtime 挂载同一个 host workspace，但容器内路径统一为 `/cnb/work`

### 15.4 参数路径统一

改造后固定为：

```text
SOURCE_GET_URL=file:///cnb/work/source/source.tgz
CNB_LAYER_ARCHIVE=/cnb/work/layers/<name>
CNB_RUN_IMAGE_ARCHIVE=/cnb/work/run/run-image.tar
PLATFORM_DIR=/cnb/work/platform
DOCKER_HOST=unix:///cnb/work/run/podman.sock  # managed
DOCKER_HOST=unix:///var/run/docker.sock           # host
PLATFORM_DIR=/platform                             # lifecycle 固定路径
```

不再把宿主机临时路径写进构建参数。

### 15.5 兼容策略

- host 模式仍然默认使用临时 workspace
- 只是内部布局改为单一根，CLI 和产物不变
- `CNB_RUNTIME_WORKSPACE` 可选；未设置时 executor 创建临时根
- 用户提供的 `BKS_RUN_IMAGE_ARCHIVE_FILE` 复制或挂载到 `<workspace>/run/run-image.tar`
- `--dest` 保持在宿主机，不进入 workspace
- cache/runtimes 仍独立挂载，不强制放进 workspace

### 15.6 代码改造

建议新增：

```text
pkg/builder/executors/workspace.go
```

职责：

```go
type Workspace struct {
    Host string
    Root string // 固定 /cnb/work
}

func (w Workspace) SourceTar() string
func (w Workspace) PlatformDir() string
func (w Workspace) LayersDir() string
func (w Workspace) RunDir() string
func (w Workspace) MountArgs() []string
```

executor：

1. 创建/确认 workspace
2. 所有中间文件写入 workspace
3. 构造根挂载 + platform 固定挂载
4. socket/cache/runtimes 作为补充挂载
5. 构建结束统一清理

### 15.7 收益

- 从多个分散挂载变成 1 个根挂载 + platform 固定挂载 + 少量可选挂载
- host / sidecar / 单容器使用同一套路径
- sidecar 不再需要手工保证 source/layers/run 等多个临时路径一致
- artifact/package 行为不变
- 更容易做回归和清理

### 15.8 验收

- 不设置新变量时，host 模式构建成功
- s-mart 结构和 artifact 不变
- sidecar 模式 builder/runtime 看到同一 `/cnb/work`
- cache/runtimes 挂载行为不变
- `BKS_EXTRA_MOUNTS` 仍可用


## 16. 路径命名的取舍：为什么不用 /bks

`/bks` 是 BK 项目前缀，不适合作为通用 builder/runtime 的路径契约。

选择：

```text
/cnb/work
```

理由：

- CNB 是两套方案共同的基础
- 不绑定 BlueKing
- 与 lifecycle 标准路径 `/workspace`、`/layers`、`/platform` 区分
- 不会被误用为应用源码目录

最终路径约定：

```text
/cnb/work/            # 编排 workspace 根
/workspace            # lifecycle 应用源码，固定
/layers               # lifecycle layers，固定
/platform             # lifecycle platform，固定
```

注意：

- `/workspace` 不能拿来当编排 workspace，lifecycle 已占用
- `/layers` 不能混入 source/platform
- `/platform` 必须单独挂载或链接
- `/cnb/work` 只放编排中间产物和 runtime 状态

环境变量建议：

```text
CNB_RUNTIME_WORKSPACE=<host path>
BKS_RUNTIME_WORKSPACE=<host path>   # 兼容别名
```

两者都只表示宿主机目录；容器内固定 `/cnb/work`。

## 17. offline-paketo 与 BK 的共同问题

两者都依赖同一个 CNB lifecycle，因此都有：

```text
/workspace
/layers
/platform
```

以及构建容器的 bind mount 语义。

区别只是：

- offline-paketo 目前没有执行器，不直接处理这些挂载
- offline-paketo-bk 有 bksmartpacker，负责实际挂载

所以问题不在“BK 包”或“非 BK 包”，而在**执行器与 workspace 契约**。

正确做法：

```text
路径契约放在共享执行层
builder 镜像只遵守 CNB 标准路径
不把 /bks 写进 builder/runtime 契约
```

如果 offline-paketo 后续也需要 managed runtime：

- 使用同一个 `/cnb/work` 契约
- 复用 runtime 镜像
- 不在 Dockerfile 中硬编码 BK 路径


## 18. 缓存路径契约

### 18.1 缓存不能放进 /cnb/work

`/cnb/work` 是单次构建 workspace，构建结束会清理。

缓存必须跨构建保留，所以不能放在：

```text
/cnb/work/cache
```

否则每次构建都会丢缓存。

缓存独立：

```text
宿主机：<cache-root>        # 持久目录
容器内：/layers/cache       # 固定挂载点
```

### 18.2 当前 host 模式

保持现有行为：

```text
<cache-root> → /layers/cache
```

构建变量仍然使用容器路径：

```text
UV_PYTHON_INSTALL_MIRROR=file:///layers/cache/pymirror
PIP_FIND_LINKS=file:///layers/cache/pkgs
```

不能在 `BKS_BUILD_ENV` 中写宿主机路径。

### 18.3 三种形态

| 形态 | cache 挂载 |
|---|---|
| host | `<cache-root>` → builder `/layers/cache` |
| sidecar | 同一 `<cache-root>` 同时挂到 builder 和 runtime `/layers/cache` |
| 单容器 | `<cache-root>` → 单容器 `/layers/cache` |

sidecar 必须两边都挂，因为构建容器由 runtime 启动，runtime 侧也要能解析
`/layers/cache` 这个路径。

### 18.4 挂载表更新

| 宿主机 | 容器 | 模式 | 条件 |
|---|---|---|---|
| `<host-work>` | `/cnb/work` | rw | 始终 |
| `<host-work>/platform` | `/platform` | rw | 始终 |
| `<cache-root>` | `/layers/cache` | rw | 配置缓存 |
| host socket | `/var/run/docker.sock` | rw | host 模式 |
| `<host-work>/run/podman.sock` | 随根挂载 | rw | managed 模式 |
| `<runtimes>` | `/cnb/runtimes` | ro | 配置 runtimes |

### 18.5 两种缓存

| 缓存 | 位置 | 用途 |
|---|---|---|
| 文件缓存 | `/layers/cache` | buildpack 下载缓存、pymirror、pip/npm 缓存 |
| registry 缓存 | `CACHE_IMAGE` | lifecycle layer cache |

两者独立：

- 文件缓存走 bind mount
- registry 缓存走 `--cache-registry` / `CACHE_REGISTRY`

### 18.6 权限与并发

- cache 目录必须对 builder 用户可写
- sidecar 模式下 runtime 也要可写
- 不要把 cache 放在 workspace 内清理

并发不是默认就要加锁，要看缓存类型：

| 缓存内容 | 并发写风险 | 建议 |
|---|---|---|
| 预置 pymirror / wheels / 只读包 | 无 | 共享 |
| Go build cache / module cache | Go 自身有锁与原子写 | 共享，无需外层锁 |
| uv cache | uv 自身做锁与原子写 | 通常共享 |
| npm cache | cacache 内容寻址，通常安全 | 可共享 |
| pip HTTP cache | 非全局锁，取决于版本与操作 | 建议按项目隔离或实测 |
| buildpack 自管可变缓存 | 取决于 buildpack | 按项目隔离 |
| registry `CACHE_IMAGE` | 同一 tag 并发导出可能冲突 | 每项目/每构建独立 tag 或串行 |

结论：

```text
Go 并发编译不需要我们额外加锁
```

只有当缓存包含：

- 非内容寻址的可变目录
- 非原子写的包管理器缓存
- 同一 registry cache tag 的并发导出

才需要：

```text
按项目分目录
或按构建分目录
或外部锁
```

推荐布局：

```text
/cache/
├── shared/          # 预置只读包、pymirror
├── projects/<app>/  # 可变的 pip/uv/npm/buildpack 缓存
└── builds/<id>/     # 必须独占的临时缓存
```

默认不要把整个 cache 做成全局可写；先区分只读与可变。

### 18.7 代码改造

建议独立抽象：

```go
type Cache struct {
    HostPath      string
    ContainerPath string // 固定 /layers/cache
}

func (c Cache) MountArgs() []string
```

不放进 `Workspace`，因为生命周期不同：

```text
Workspace: 单次构建
Cache:     跨构建
```

### 18.8 验收

- host 模式缓存命中行为不变
- sidecar 模式 builder/runtime 都能看到 `/layers/cache`
- `BKS_BUILD_ENV` 不需要写宿主机路径
- 构建结束只清理 `/cnb/work`，不清理 cache
- registry cache 与文件 cache 可同时使用


## 19. 完整缓存目录结构

### 19.1 根路径

```text
宿主机：<cache-root>              # 按目标架构分开
容器内：/layers/cache
CNB_CACHE_DIR=/layers/cache
```

推荐宿主机按架构分开：

```text
/var/lib/cnb/cache/linux-amd64/
/var/lib/cnb/cache/linux-arm64/
```

不要用一个 cache 根同时服务 amd64/arm64。

### 19.2 目录树

```text
/layers/cache/
├── shared/                         # 只读、可跨项目共享
│   ├── pymirror/                   # python-build-standalone 镜像
│   ├── wheels/                     # 本地 wheel / sdist
│   └── runtimes/                   # 预置 Go/Node/Python 运行时
│
├── python/
│   ├── install/                    # cpython 解释器安装
│   └── uv/                         # uv 共享缓存
│
├── go/
│   ├── sdk/                        # go<version> 安装目录
│   ├── download/                   # Go SDK 下载 tar
│   ├── mod/                        # GOMODCACHE
│   └── build/                      # GOCACHE
│
├── node/
│   └── node-versions/              # fnm 安装的 Node
│
├── npm-cache/                      # npm 包缓存
│
├── pip/                            # 可选 pip HTTP cache
│
└── projects/                       # 可变缓存，按项目隔离
    └── <app-code>/
        ├── pip/
        ├── npm/
        └── buildpack/
```

### 19.3 与当前 buildpack 的对应

| 缓存 | buildpack | 当前路径 | 目标路径 |
|---|---|---|---|
| Python 解释器 | cpython | `/layers/cache/python/install` | 同 |
| uv 缓存 | cpython | `/layers/cache/python` | `/layers/cache/python/uv` |
| uv 缓存 | pip-install | `/layers/cache/python/uv` | 同 |
| Go SDK | go-dist | `/layers/cache/go/sdk` | 同 |
| Go 下载 | go-dist | `/layers/cache/go/download` | 同 |
| Go module | go-build | `/layers/cache/go/mod` | 同 |
| Go build | go-build | `/layers/cache/go/build` | 同 |
| Node | node-engine | `/layers/cache/node` | 同 |
| npm | npm-install | `/layers/cache/npm-cache` | 同 |
| pymirror | cpython | `/layers/cache/pymirror` | `/layers/cache/shared/pymirror` |
| 本地 wheels | pip-install | `/layers/cache/pkgs` | `/layers/cache/shared/wheels` |

迁移时可以保留旧路径兼容：

```text
shared/pymirror -> ../pymirror
shared/wheels   -> ../pkgs
```

### 19.4 环境变量

```text
CNB_CACHE_DIR=/layers/cache

CPYTHON_CACHE=/layers/cache/python/install
UV_CACHE_DIR=/layers/cache/python/uv
GOMODCACHE=/layers/cache/go/mod
GOCACHE=/layers/cache/go/build
FNM_DIR=/layers/cache/node
npm_config_cache=/layers/cache/npm-cache
PIP_CACHE_DIR=/layers/cache/pip
PIP_FIND_LINKS=/layers/cache/shared/wheels
UV_PYTHON_INSTALL_MIRROR=file:///layers/cache/shared/pymirror
```

### 19.5 最大化利用率

- 工具链只安装一次，按版本放 `go/sdk`、`python/install`、`node/node-versions`
- 包管理器缓存集中到 `go`、`python/uv`、`npm-cache`、`pip`
- 只读镜像/ wheel 放 `shared/`
- 可变缓存按项目放 `projects/<app>/`
- workspace 只放单次构建文件，不清理 cache
- registry cache 用独立 tag，不和文件缓存混用
- 预热 cache 可用离线 bundle 的 pymirror/wheels/runtimes

### 19.6 挂载关系

```text
host <cache-root> → /layers/cache
```

- host 模式：挂到 builder
- sidecar：同时挂到 builder 和 runtime
- 单容器：挂到单容器

不要放进 `/cnb/work`。


## 20. 完整缓存结构的遗漏项

### 20.1 包管理器缓存

当前 npm-install 支持：

```text
npm
pnpm
yarn
```

但只显式设置了 npm cache。需要补：

| 工具 | 缓存路径 | 配置 |
|---|---|---|
| npm | `/layers/cache/npm-cache` | `npm_config_cache` |
| pnpm | `/layers/cache/pnpm-store` | store-dir / 对应环境变量 |
| yarn classic | `/layers/cache/yarn` | `YARN_CACHE_FOLDER` |
| corepack | `/layers/cache/corepack` | `COREPACK_HOME` |

否则 pnpm/yarn 会回落到 `~/.cache` 或项目目录，跨构建无法复用。

### 20.2 Python 缓存

| 缓存 | 路径 |
|---|---|
| uv | `/layers/cache/python/uv` |
| pip | `/layers/cache/pip` |
| interpreter | `/layers/cache/python/install` |
| wheels | `/layers/cache/shared/wheels` |

### 20.3 managed runtime storage

不能放在 `/cnb/work/runtime`，因为 workspace 每次清理。

应放到持久 cache：

```text
/layers/cache/runtime/podman/<arch>/storage
/layers/cache/runtime/docker/<arch>/data-root
```

socket/pid 仍放：

```text
/cnb/work/run/
```

区别：

```text
runtime storage = 跨构建缓存
runtime socket  = 单次构建
```

### 20.4 临时目录

不要把临时目录放进 cache：

```text
TMPDIR=/cnb/work/tmp
GOTMPDIR=/cnb/work/tmp
UV_TMPDIR=/cnb/work/tmp       # 如果版本支持
```

否则缓存会被临时文件污染。

### 20.5 遗漏清单

- [ ] pnpm store
- [ ] yarn cache
- [ ] corepack home
- [ ] pip HTTP cache
- [ ] managed runtime storage
- [ ] custom run image cache
- [ ] 缓存锁目录（仅必要项目）
- [ ] 权限与属主（rwx / 属组）
- [ ] 多架构目录隔离（工具链必须，Go module 可共享）
- [ ] 可选预热脚本
- [ ] registry cache tag 隔离

说明：容量/淘汰由宿主机磁盘运维负责，不在工具内实现。

### 20.6 结论

```text
缓存不只是语言工具链
还包括：
  包管理器
  嵌套运行时 storage
  run image
  临时目录隔离
  锁与淘汰
```

主结构已经覆盖工具链和语言缓存，剩余重点是把 pnpm/yarn/corepack、runtime storage 和临时目录补齐。


## 21. 缓存治理的边界

### 21.1 容量与淘汰

缓存最终落在宿主机磁盘，工具不负责全局淘汰。

原因：

```text
宿主机容量、挂载点、清理策略属于运维
工具只负责写入约定目录
```

需要保证：

- 缓存目录可删除
- 删除后构建可重新下载/编译
- 不把唯一产物放缓存

不需要在工具内实现 LRU / TTL / 配额。

### 21.2 权限与属主

目录需要 rwx，文件按类型：

| 类型 | 权限 |
|---|---|
| 普通缓存文件 | `0644` |
| 可执行工具 | `0755` |
| 缓存目录 | `0775` + setgid `2775` |
| shared 只读目录 | `0555/0755` |

不要用 `0777`。

建议：

- cache 根属组为 `cnb` 或 builder/runtime 共同组
- 目录 setgid，新增文件继承组
- sidecar 的 builder 与 runtime 使用相同组
- 不依赖 world-writable

### 21.3 多架构

必须隔离：

```text
Go SDK
Python interpreter
Node binary
runtime storage
run image
```

可以共享：

```text
Go module source
pymirror 的纯源码包
shared wheels 中与架构无关的包
```

推荐：

```text
/cache/linux-amd64/
/cache/linux-arm64/
```

Go 的 `GOCACHE` 条目按构建输入区分，通常可共享；但工具链 SDK 必须按架构隔离。

### 21.4 预热脚本

预热脚本不是构建必需，是**可选的加速手段**。

作用：

```text
在构建前把固定版本的工具链/包下载到 cache
避免每次构建首次联网
```

典型内容：

- Go SDK
- Python interpreter
- Node
- pymirror / wheels
- 固定版本的 pnpm/yarn/corepack

执行时机：

- CI 预热阶段
- 镜像构建阶段
- 定时任务

不做：

- 每次构建都跑
- 预热脚本内做业务构建

### 21.5 缓存锁

默认策略：

```text
不加全局锁
```

依赖：

- Go / uv / npm / pnpm 自带并发控制
- 按项目隔离可变缓存
- registry cache 使用独立 tag

这里的“并发”指：

```text
两个独立构建容器
挂载同一个宿主机 cache 根
同时写同一个缓存子目录
```

例如：

```text
build A: /host/cache/projects/app-a/pip
build B: /host/cache/projects/app-a/pip
```

或者：

```text
build A: /host/cache/python/uv
build B: /host/cache/python/uv
```

不是“多个 Go 编译共享 GOCACHE”。

只有在以下情况才加锁：

- 多个构建并发写同一个 pip/buildpack 可变目录
- 多个构建共享同一个 registry cache tag
- 自定义脚本对同一缓存目录做非原子修改

锁方案：

```text
flock / 原子 mkdir
锁文件：/layers/cache/locks/<scope>.lock
超时 + stale 检测
```

不要：

- 锁只读 shared
- 锁 Go module/build cache
- 在 NFS 上依赖 flock

### 21.6 缓存完整性

包管理器自身的 hash/锁文件校验不属于本工具职责。

本工具只需要保证：

- 缓存目录可删除、可重建
- 不把唯一产物放缓存
- 不假设缓存一定完整

原子写只在**我们自己的缓存填充脚本**里有意义：

| 流程 | 是否需要我们处理 |
|---|---|
| Go/uv/npm/pnpm 写自己的缓存 | 否，工具自身处理 |
| buildpack 下载进 cache | 否，缓存可重建 |
| 预热脚本下载工具链 | 是，建议 temp + rename |
| 预热脚本解包 wheel/runtime | 是，建议解包到临时目录后 rename |
| 构建产物写 workspace | 否，单次构建独占 |

构建时如果 cache 损坏：

```text
删除对应缓存目录
重新构建
```

不需要全量 checksum。

### 21.7 结论

```text
容量/淘汰：宿主机运维
权限：rwx + 属组，不用 0777
多架构：工具链必须隔离，Go module 可共享
预热：可选加速
锁：默认不加，必要时 flock/原子 mkdir
完整性：依赖包管理器校验 + 原子写 + 版本键
```
