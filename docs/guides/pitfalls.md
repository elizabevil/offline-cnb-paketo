# 踩坑记录（构建与运行时）

> 正在排查线上 Pod 时，先看 [troubleshooting.md](troubleshooting.md)。

> 每条都是实际发生过的故障：**现象 → 原因 → 修法**。
> 按"先看现象"组织，避免下次再从上层往下猜。

## 一、运行时报"文件不存在"，但文件确实在

### 1. `env: can't execute 'node': No such file or directory`

| | |
|---|---|
| 现象 | 镜像里 `bin/node` 是 98MB 实体文件、权限正确，容器启动仍报不存在 |
| 原因 | **run 镜像与二进制 libc 不匹配**。node 是 glibc 链接，run 镜像是 alpine（musl），`execve` 找不到 `ld-linux` 时报 ENOENT |
| 修法 | run 镜像改用 glibc 基础（`debian:trixie-slim`） |

> 判断顺序：先 `ldd <binary>`，再看 PATH。**"文件存在但报不存在"九成是动态链接器**。
>
> 这条规律在三个项目上都复现过（offline-paketo、offline-paketo-bk）；
> 目前没有自动检查：base 镜像的 libc 与运行时需人工确认（alpine/musl 不能跑 Node/Python 官方构建）
> 运行时即报错，不必等容器起不来。

官方 Node 只有 glibc 构建（nodejs.org 与 fnm 均是），无静态版；
Python（uv 的 `cpython-*-gnu`）同理；Go 静态链接，alpine 也可。

### 2. `npm: command not found`（exit 127）

| | |
|---|---|
| 现象 | 层里有 `bin/npm`，启动仍找不到 |
| 原因 | ①启动阶段没有 PATH；②软链指向 cache（cache 不进镜像）→ 悬空 |
| 修法 | 启动命令内联 PATH；launch 层必须**自包含**（复制而非软链到 `/layers/cache`） |

## 二、lifecycle 的路径与契约

### 3. `layers_dir` 是 buildpack 私有目录

```
layers_dir        = /layers/<buildpack-id>     ← 私有
dirname(layers_dir) = /layers                  ← 全局，跨组件查找用这个
```

| | |
|---|---|
| 现象 | 组件间找不到对方的层（如 npm-install 找不到 node 层） |
| 原因 | 用 `${layers_dir}/*/node/bin` 查找，实际应在 `/layers/*/node/bin` |
| 修法 | `root="$(dirname "${layers_dir}")"`，不要依赖 `CNB_LAYERS_DIR`（可能未设置） |

### 4. 层元数据必须是 `<layer>.toml` + `[types]`

```toml
# ✗ 旧格式，lifecycle 不认
<layer>.launch: launch = true

# ✓
<layer>.toml:
[types]
launch = true
build  = true
cache  = true
```

后果：layer 不被视为 launch 层 → `env/PATH.prepend` 不生效、层不进镜像。

### 5. `launch.toml` 属于 buildpack 自己的目录

```
<layers>/<buildpack-id>/launch.toml     ← lifecycle 按 buildpack 聚合
```

写到顶层 `/layers/launch.toml` 反而读不到。

### 6. `processes.command` 的类型取决于 buildpack api

| api | `command` 形式 |
|---|---|
| 0.8 | 字符串 `"npm start"` |
| 0.12 | 数组 `["npm","start"]`；需要 shell 时 `["/bin/sh","-c","…"]` |

> 两者混用会报 `TOML value has type string; destination has type slice`（或反向）。
> 结论：**自研 buildpack 统一 api 0.12**。

### 7. `layers` 必须每次全新，只有 `cache` 持久

| 现象 | 原因 |
|---|---|
| `rename /layers/<bp>/launch → launch.ignore: file exists` | 复用了上次的 layers |

约定：

```
/layers          每次全新
/layers/cache    跨构建保留（唯一持久）
```

### 8. `/workspace` 需要可写

`npm install` 会写 `package-lock.json`，挂 `:ro` 会报 `EROFS`。
复制一份可写副本挂载，保持 fixture 干净。

## 三、工具与环境

### 9. `fnm install` 已安装时返回非 0

`set -e` 下会静默中断（日志停在 "Version already installed"）。
修法：`fnm install … || true`，以"二进制是否存在"为准。

### 10. fnm 的目录带 `v` 前缀

```
node-versions/v20.20.2/installation/bin   ← 实际
node-versions/20/installation/bin         ← 按配置主版本猜的
```

修法：按主版本前缀 glob：`node-versions/v?<major>(\.[0-9]+)*/`。

### 11. 归档名不含版本会复用旧包

`yq_linux_amd64`、`jq-linux-amd64`、`uv-x86_64-…tar.gz` 都不含版本，
升级后会把**旧包哈希固定到新版本**上。

修法：归档按 `<id>/<version>/<basename>` 存放。

### 12. GitHub 发布下载可能被阻断（API 通、release 不通）

修法：`GH_MIRROR` 前缀（如 `https://gh-proxy.com/`；`ghfast.top` 对部分产物 403）。

## 四、pack 与平台兼容

### 13. pack 需要的 builder 契约

| 报错 | 原因 | 修法 |
|---|---|---|
| `missing required env var 'CNB_USER_ID'` | builder 未声明构建用户 | `ENV CNB_USER_ID=1000` / `CNB_GROUP_ID=1000` |
| `invalid run-image '': run image must be specified` | metadata 缺 runImage | 构建时 `--label io.buildpacks.builder.metadata=…` 或显式 `--run-image` |
| `image 'docker.io/library/x' does not exist` | podman 名字带 `localhost/` | 打规范 tag（`docker.io/library/x:tag`） |
| `permission denied … docker.sock` | rootless podman 的 userns 映射 | 改用 lifecycle 直连，或 `--userns=keep-id` / rootful |

### 14. pack 的短名会被规范化

`--builder offline-builder` → `docker.io/library/offline-builder`。
先 `podman tag <本地名> docker.io/library/<名>:<tag>`，再 `--pull-policy never`。

## 五、配置与仓库

### 15. `order.toml` 被覆盖

Dockerfile 里同时 `COPY --from=bundle …/order.toml` 和 `COPY docker/order.toml`，
后者覆盖前者，导致 detect 找不到对应 id 的目录。

> 规则：order 只能有**一个**来源。切换实现方式时要删掉旧的 COPY。

### 16. buildpack 版本必须在四处同步

| 位置 | 后果（漏改时） |
|---|---|
| `buildpack.toml` version | 事实源 |
| Dockerfile `/cnb/buildpacks/<id>/<version>/` | 目录名不对 → 找不到 |
| `order.toml` 的 version | lifecycle 按此查找 → 找不到 |
| e2e 新鲜度断言 | 误报"镜像过期" |

`cnb buildpack lint` 会检查前两项与目录结构。

### 17. 断言本身会过期

e2e 的新鲜度断言写死了路径，组件拆分后仍检查旧路径 → 误报。
断言应指向**当前结构**，与组件拆分同步更新。

### 18. buildpacks 双份副本会漂移

两个项目各存一份 `docker/buildpacks/` 时，改一处忘一处几乎必然发生
（`docker/layers` 的残留 COPY 就是痕迹）。

现在 `buildpacks/` 位于仓库根并按格式分组（`paketo/`、`heroku/`，后续可加），
项目用 `[docker] buildpacks_context` 声明用哪一组；Dockerfile 通过
`COPY --from=buildpacks /<component>/ …` 引用，`cnb pipeline run` 传
`--build-context buildpacks=<repo>/buildpacks`。

### 19. 本地构建产物也要走物料，别硬编码路径

`cnb` 这类由本仓库编译的工具，同样按物料管理：

```toml
[asset.platform.linux-amd64]
kind   = "local"
source = "script/bin/cnb-linux-amd64"   # 构建产物路径（相对仓库根）
sha256 = ""                             # 发布后自动回填
```

- `just tools` 编译后自动执行 `cnb materials publish cnb`，复制到
  `bundle/<platform>/binaries/cnb/<version>/cnb` 并回填哈希
- 本地物料的哈希是**构建指纹**，每次发布覆盖（不是下载物的校验语义）

## 附：脚本层的坑（来自基座早期）

| 坑 | 现象 | 处置 |
|---|---|---|
| `just` 的路径函数语义 | import 后 `justfile_directory()` 指向**仓库根**，不是被 import 的文件 | 基座内部一律用 `source_directory()` |
| `#!/bin/sh` + `set -o pipefail` | Debian 12 以前 `/bin/sh`(dash<0.5.12) 直接报错退出 | shell 脚本统一 `#!/usr/bin/env bash`；或确认基础镜像 dash ≥ 0.5.12 |
| `tar --strip-components=1` | 归档根部的单文件（如 `lifecycle.toml`）被静默丢弃 | 不要把根级文件当作必需产物；用 `expect` 显式声明 |
| `stat -c%s` vs `stat -f%z` | Linux/macOS 参数不同 | `common.sh` 里已做双分支兜底 |
| `unzip --version` | 会先打印 `caution: both -n and -o specified` | 自检输出里只取首行，属正常噪音 |
| `set -o pipefail` + `\| head -n1` | 上游写满管道被关闭 → SIGPIPE → 退出码 141，`set -e` 直接终止脚本 | 管道末尾加 `\|\| true`，或改用 `awk 'NR==1'` |
| 只读 `HOME` 下 `go build` | `open ~/.cache/go-build/...: read-only file system` | 显式 `GOCACHE=<仓库>/.cache/go-build` |
| `bundle/<platform>/binaries` 手工改文件 | 下次 fetch 按标记重解压会覆盖 | 不要在 `binaries/` 下做任何手工修改，改 lock 或改脚本 |

---

## 六、运行时与 pind/dind

### 17. qemu-user 下 Podman 报 `function not implemented`

| | |
|---|---|
| 现象 | arm64 pind sidecar 启动失败：`failed to create 2048 locks in /libpod_lock: function not implemented` |
| 原因 | Podman 默认 shm lock 使用 robust process-shared pthread mutex；qemu-user 不完整支持 robust futex / `set_robust_list` |
| 修法 | 真机/arm64 CI 验证；临时探测可用 `[engine] lock_type = "file"` |
| 影响 | 不是 pind 镜像或 podman-static 缺依赖；qemu-user 的限制 |

参考：

- [runtime-matrix.md](../design/components/bksmartpacker/runtime-matrix.md)
- [runtime-concepts.md](../design/runtime-concepts.md)

### 18. lifecycle `singleton connection already in use`

| | |
|---|---|
| 现象 | Podman Docker-compatible API + exporter `-daemon`，导出 base layers 时报 singleton |
| 原因 | lifecycle `priv/docker.go` 复用一条预打开连接；root-owned socket 时进入单连接路径，第二个请求无法 dial |
| 修法 | ① Podman 优先 `layout`；② daemon 模式使用非 root builder + `keep-id`；③ 等 lifecycle #1763 修复 |
| dind 备注 | dind 也允许显式 `layout`；默认仍是 daemon |
| pind 备注 | pind 也支持 daemon，使用 Podman Docker-compatible API |
| 注意 | Podman 服务端本身并发 `image get` 正常，问题在 lifecycle 客户端 |

上游：

- https://github.com/buildpacks/lifecycle/issues/1763

### 19. dind 在 root builder 下同样触发 singleton

| | |
|---|---|
| 现象 | `runtime-dind` + root builder 在 analyze/export 阶段失败 |
| 原因 | 与 Podman 场景共享 lifecycle 单连接机制；builder 内 socket 属主为 root |
| 修法 | 使用 `BKS_BUILDER_USER=1000:1000`、`BKS_BUILDER_USERNS=keep-id` |
| 前提 | builder 用户对 workspace/cache 可写；socket 在 builder 内可见且可连接 |

### 20. 不要把 Docker 静态物料重复登记

| | |
|---|---|
| 现象 | 曾新增 `docker-static` asset，与已有 `docker` asset 重复 |
| 原因 | 已有 `docker` asset 本身就是静态 Docker 包，包含 dockerd/containerd/runc/docker CLI |
| 修法 | runtime-dind 直接复用 `bundle/<platform>/binaries/docker/<version>/docker/`；不依赖 `docker:27-dind` 镜像 |
| 教训 | 新增 asset 前先查 `project/assets.toml` |

### 21. arm64 Python buildpack 仍可能下载 x86_64 运行时

| | |
|---|---|
| 现象 | `BKS_PLATFORM=linux/arm64` 下 cpython buildpack 下载 `cpython-*-linux-x86_64-gnu` |
| 原因 | 虽然注入了 `CNB_TARGET_ARCH=arm64`，但当前 cpython/uv 下载路径没有完全按目标架构选择 |
| 状态 | 未修复；arm64 Go 构建已验证，arm64 Python 需要单独修 buildpack |
| 影响 | 不是 dind/pind 运行时问题，是 arm64 buildpack 支持问题 |

### 22. 测试产物必须带方案/架构

| | |
|---|---|
| 现象 | 过去测试统一生成 `s-mart.tar.gz`，无法区分 host/pind/dind、amd64/arm64 |
| 修法 | 测试产物命名为 `s-mart-<runtime>-<export>-<arch>.tar.gz`，例如 `s-mart-pind-layout-amd64.tar.gz` |
| 位置 | `.cache/tests/full/<platform>/`、`.cache/tests/offline-paketo-bk/<platform>/` |
