# 踩坑记录（构建与运行时）

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
> 现在由 `cnb doctor` 静态检查：run 镜像含 alpine/musl 且项目含 Node/Python
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
`COPY --from=buildpacks /<component>/ …` 引用，`cnb run` 传
`--build-context buildpacks=<repo>/buildpacks`。
