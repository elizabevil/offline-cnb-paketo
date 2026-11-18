# bksmartpacker_v3 fork 说明

`script/tools/bksmartpacker_v3` 不是独立实现，而是蓝鲸官方
`bksmartpacker_v3` 的 **fork**。本页记录差异、兼容性与同步方式，避免
每次升级都重新 diff。

## 一、基线

| 项 | 值 |
|---|---|
| 上游仓库 | `ssh://git@code.cwoa.net:43990/rd-fy22-canway-platform-products/bksmartpacker_v3.git` |
| 基线提交 | `e2025de feat: 工具构建切换静态链接` |
| 本地差异 | 4 个文件修改 + 3 个新增（不含 go.sum、工具产物） |
| 变更规模 | 约 450 行 |

上游工作区自带一个未提交的 `Makefile` 改动（产物名加 `-ts4`），本 fork
已包含；这不算我们的差异。

## 二、差异清单

| 文件 | 差异 | 原因 |
|---|---|---|
| `pkg/builder/executors/docker.go` | layout 导出、podman 支持、`--platform`、run image 文件挂载、确定性打包 | podman + 离线 + arm64 |
| `pkg/builder/executors/constants.go` | fake runner 按架构打 tag | 避免 amd64/arm64 互相覆盖 |
| `cmd/bksmartpacker/main.go` | 新增 `--inspect` / `--inspect-format`；默认 dest 与官方一致 | 包结构自检 |
| `pkg/log/log.go` | `Println(v)` → `Println(v...)` | 修复切片打印 |
| `pkg/smartpkg/`（新增） | 解析 s-mart 包并输出摘要 | `--inspect` 的实现 |
| `pkg/utils/pack.go`（新增） | 属主 0:0、固定 mtime、权限归一 | 打包可复现 |
| `justfile`（新增） | 交叉编译入口 | 产出 `script/bin/bksmartpacker-<os>-<arch>` |

以下内容**与上游一致**，升级时一般不需要动：

- `pkg/builder` 的 plan / buildpack / source tarball 逻辑
- `pkg/types`、`pkg/config`、`pkg/cmd`
- `conf/buildpack.yaml`
- s-mart 包协议：`.Version=cnb-image-layers` + `app_desc.yaml` +
  `artifact.json` + `<module>.tgz`

## 三、行为差异

### 运行时

| 能力 | 上游 | fork |
|---|---|---|
| 容器运行时 | 硬编码 `docker` | `BKS_CONTAINER_CLI`，缺省依次尝试 docker、podman |
| socket | 固定 `/var/run/docker.sock` | `BKS_DOCKER_SOCK` 可覆盖 |
| 导出方式 | daemon only | layout（默认）/ daemon（`BKS_EXPORT_MODE`） |
| fake runner | `FROM scratch` 构建 | `image import` 构造单层镜像 |
| 目标平台 | 不支持 | `BKS_PLATFORM=linux/arm64` |

### 打包

| 项 | 上游 | fork |
|---|---|---|
| tar 元数据 | archiver 默认（含宿主属主/时间） | 属主 0:0、固定 mtime、权限归一 |
| 包自检 | 无 | `--inspect <pkg>`，支持 `text` / `json` |
| 默认 dest | `./s-mart.tgz` | `./s-mart.tgz`（与上游一致） |

## 四、环境变量

| 变量 | 作用 |
|---|---|
| `BKS_CONTAINER_CLI` | 指定容器命令（`docker` / `podman`），缺省自动探测 |
| `BKS_DOCKER_SOCK` | 传给容器的 socket 路径（podman 用 `/run/user/<uid>/podman/podman.sock`） |
| `BKS_EXPORT_MODE` | `layout`（默认）/ `daemon` |
| `BKS_PLATFORM` | 目标平台，如 `linux/arm64`；同时决定 fake runner tag |
| `BKS_RUN_IMAGE_ARCHIVE_FILE` | 以 docker-archive 形式挂载 run image，容器内无需 daemon |
| `BKS_EXTRA_MOUNTS` | 追加 `--mount` 参数（如 runtimes 目录） |
| `BKS_NETWORK` | 传给 `docker run` 的网络模式 |
| `BKS_BUILDPACK_ALIASES` | 覆盖默认 buildpack 别名（注入 `CNB_BUILDPACK_ALIASES`） |

## 五、兼容性

| 场景 | 结论 |
|---|---|
| 用 fork 替换上游 | ✅ 参数、配置、包协议兼容；默认 dest 已对齐 |
| 上游脚本调用 fork | ✅ 未知参数不存在，行为不变 |
| fork 脚本调用上游 | ⚠️ 不能使用 `--inspect`、`BKS_*`；podman/arm64 不可用 |
| 蓝鲸平台消费产物 | ✅ 已实际部署验证（含 arm64） |

已知限制：

- daemon 导出在 podman 下不可用：lifecycle 0.21.29 报
  `singleton connection already in use`（upstream issue #1763，未修复）。
  该模式仅为 docker 节点保留，podman 下必须用 layout。
- 上游没有测试覆盖 executor，升级后需要手工跑一次 amd64/arm64 打包。

## 六、升级上游

1. 拉取上游新提交，查看是否触及差异文件：

   ```bash
   git -C <upstream> log --oneline e2025de..HEAD -- \
     pkg/builder/executors pkg/types pkg/config pkg/cmd conf
   ```

2. 以新提交为基线，重放本 fork 的差异（建议按文件 cherry-pick 手工合并）。
3. 更新本页的「基线提交」与差异规模。
4. 重新构建并验证：

   ```bash
   just --justfile script/tools/bksmartpacker_v3/justfile build
   cd project/offline-paketo-bk
   CNB_BIN=../../script/bin/cnb-linux-amd64 \
   BPS_BIN=../../script/bin/bksmartpacker-linux-amd64 \
   just smart /path/to/release-amd64
   ../../script/bin/bksmartpacker-linux-amd64 --inspect dist/linux-amd64/s-mart.tar.gz
   ```

## 七、构建

```bash
just --justfile script/tools/bksmartpacker_v3/justfile build
# 交叉编译
TARGET=linux-arm64 just --justfile script/tools/bksmartpacker_v3/justfile build
```

产物固定输出到 `script/bin/`，源码目录不入库。
