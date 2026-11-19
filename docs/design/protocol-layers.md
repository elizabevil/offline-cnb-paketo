# 协议分层方案

> 状态：已定稿（2026-11）。**不兼容历史遗留**：旧标记、旧脚本、旧双解析器一律删除，不做兼容层。

## 一、要解决的问题

当前 cnb 同时解析 6 类协议，但边界不清：

| 症状 | 实例 |
|---|---|
| 同一协议两套解析器 | `order.toml` 在 `lifecycle/order.go`（toml）与 `checks/buildpack.go`（正则）各解析一次 |
| 格式知识分散 | docker-archive 在 `archive`/`repack`/`smartpkg` 三处；OCI layout 在 `repack` 与 `smartpkg` |
| 平台契约外溢 | `CNB_*` 在 cmd，`SOURCE_GET_URL` 在 lifecycle，BK 别名在 cmd |
| 双协议并存 | 物料标记 `.sha256`（旧）与 `.sync.json`（新） |
| 无最终值视图 | 同一字段可被 lock / profile / env / flag 覆盖，无从得知生效值 |
| 阶段类型靠猜 | 旧 profile 用 `image=` / `command=` 的有无判断类型，新增 shell/python 需改解析器 |

## 二、分层模型

```
spec/manifest   物料清单（本项目自有）
spec/profile    流水线编排（本项目自有）
spec/lifecycle  CNB lifecycle 的 6 个文件协议
spec/platform   平台 → buildpack 契约（环境变量 / flags）
spec/imagefmt   镜像归档格式（docker-archive / OCI layout）

（平台私有）     BK 的 app_desc / artifact.json / s-mart → 只在 bksmartpacker 内
```

依赖方向：业务包 → spec；spec 之间不互相依赖（`platform` 可依赖 `lifecycle` 的枚举，单向）。

## 三、四条硬规则

1. **单一解析器**：每个协议只有一处 `Parse`；其他包只能消费结构体，不得再解析文本。
2. **写方向唯一**：`Parse` 与 `Encode` 同包；谁写谁校验（例如 sha256 只能由 `materials` 回填）。
3. **未知键必须报告**：解析器遇到未知字段返回 `Warn` 列表（不是静默忽略），协议升级必须显式处理。
4. **平台私有协议不得进入通用包**：`BKS_*`、`app_desc.yaml`、`.Version` 不出现于 `internal/`。

## 四、包结构与 API 形状

每个 spec 包只做三件事：**解析、编码、校验**。不做 I/O 调度、不读环境变量、不打印。

```go
package manifest

type File struct { Schema int; Storage Storage; Assets []Asset }
type Storage struct { Root string; Layout Layout }

func Parse(data []byte) (*File, []warn.Warning, error)
func (f *File) Encode() ([]byte, error)          // 仅用于生成模板
func (f *File) Validate() []error
```

约定：

- 解析函数统一签名 `Parse([]byte) (T, []warn.Warning, error)`，文件读取由调用方完成
- 校验返回 `[]error`（可一次报多条），不返回 bool
- 不导出可变字段以外的行为（无 `Load`、无 `Save`，路径由业务层决定）

## 五、各层协议案例

### 5.1 spec/manifest —— `assets.lock.toml`

```toml
schema = 1

[storage]
root   = "bundle"
layout = "arch-first"

[asset.info]
description = "CNB lifecycle 四阶段二进制"
usage       = "lifecycle run / exporter / analyzer"

[asset.unpack]
kind   = "tgz"
strip  = 1
dir    = "lifecycle/{version}"
expect = ["lifecycle", "detector", "analyzer", "exporter"]

[[asset]]
id      = "lifecycle"
version = "0.21.29"
default = true

  [asset.platform.linux-amd64]
  url    = "https://github.com/buildpacks/lifecycle/releases/download/v0.21.29/lifecycle-v0.21.29+linux.x86-64.tgz"
  sha256 = "…"
  size   = 12345678
```

职责：

- 结构体：`File` / `Storage` / `Asset` / `Info` / `Unpack` / `Source`
- 类型化字段：`Kind`（tgz/tar.gz/zip/bin）、`Platform`（linux-amd64…）、`Layout`
- 校验：schema、id 唯一、平台白名单、url/sha256/size 配对、`dir` 冲突
- 编辑（保留注释）留在 `tomledit`，由 `materials` 调用

### 5.2 spec/profile —— `config/projects/<name>.toml`（K8s 结构 + 统一 exec）

**格式用 TOML，结构照 K8s**：`apiVersion` / `kind` / `metadata` / `spec`。
`spec.stages` 是**列表**，顺序即执行顺序；每个 stage 只有一条命令，没有类型分派。

```toml
apiVersion = "cnb/v1"
kind       = "Pipeline"

[metadata]
name = "offline-paketo-bk"

[spec.project]
dir      = "project/offline-paketo-bk"
platform = "linux-amd64"

# ── 物料：本项目锁定的版本（省略则取 lock 的 default）────────
[spec.assets.lifecycle]
version = "0.21.29"

[spec.assets.fnm]
version = "1.39.0"

[spec.assets.cnb]
version = "3.0.0"

# ── 阶段：列表顺序 = 执行顺序 ───────────────────────────────

# ── 物料版本 ───────────────────────────────────────────────
[spec.assets.lifecycle]
version = "0.21.29"

[spec.assets.fnm]
version = "1.39.0"

[spec.assets.uv]
version = "0.13.8"

[spec.assets.docker]
version = "27.0.3"

[spec.assets.jq]
version = "1.8.2"

[spec.assets.yq]
version = "4.54.3"

[spec.assets.pack]
version = "0.40.11"

[spec.assets.cnb]
version = "3.0.0"

[[spec.stages]]
name    = "base"
workdir = "."
run = [
  "podman", "build",
  "-f", "project/offline-paketo-bk/docker/base.dockerfile",
  "-t", "offline-builder-base-bk:latest",
  "--build-arg", "BASE_IMAGE=debian:trixie-slim",
  "--build-context", "bundle=bundle/linux-amd64",
  ".",
]

[[spec.stages]]
name    = "builder"
workdir = "."
run = [
  "podman", "build",
  "-f", "project/offline-paketo/docker/builder.dockerfile",
  "-t", "offline-builder:latest",
  "--build-context", "bundle=${bundle}/${platform}",
  "--build-context", "buildpacks=buildpacks/paketo",
  "--build-arg", "BASE_IMAGE_TAG=offline-builder-base:latest",
  "--build-arg", "PROCFILE_BP_TAG=swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/paketobuildpacks/procfile:5.13.4",
  "--build-arg", "LIFECYCLE_VERSION=${asset.lifecycle.version}",
  "--build-arg", "FNM_VERSION=${asset.fnm.version}",
  "--build-arg", "UV_VERSION=${asset.uv.version}",
  "--build-arg", "CNB_VERSION=${asset.cnb.version}",
  ".",
]
```

### A.2 offline-paketo-bk（BK 契约，3 个镜像）

```toml
apiVersion = "cnb/v1"
kind       = "Pipeline"

[metadata]
name = "offline-paketo-bk"

[spec.project]
dir      = "project/offline-paketo-bk"
platform = "linux-amd64"

[[spec.stages]]
name    = "base"
workdir = "."
run = [
  "podman", "build",
  "-f", "project/offline-paketo-bk/docker/builder-base.dockerfile",
  "-t", "offline-builder-base-bk:latest",
  "--build-arg", "BASE_IMAGE=swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/debian:trixie-slim",
  ".",
]

[[spec.stages]]
name    = "builder"
workdir = "."
run = [
  "podman", "build",
  "-f", "project/offline-paketo-bk/docker/builder-build.dockerfile",
  "-t", "offline-builder-build-bk:latest",
  "--build-context", "bundle=${bundle}/${platform}",
  "--build-context", "buildpacks=buildpacks/paketo",
  "--build-arg", "BASE_IMAGE_TAG=offline-builder-base-bk:latest",
  "--build-arg", "BP_GO_MIRROR=http://192.168.11.4:5000/",
  "--build-arg", "BUNDLE_ARCH=${platform}",
  ".",
]
```

### A.4 旧结构里消失的东西

| 旧字段 | 去处 |
|---|---|
| `schema = 1` | `apiVersion = "cnb/v1"` |
| `[project]` / `[docker]` | `spec.project`；`bundle_layout` 由 lock `[storage].layout` 决定 |
| `[assets.*]` | 改为 `spec.assets.*`：仍按项目锁版本，省略取 lock default |
| `[images.*]` | 删除：直接写进 `spec.stages[].run` |
| `[commands.*]` | 删除：同上 |
| `inject_versions` | 删除：改为在 `run` 里显式 `--build-arg X_VERSION=${asset.x.version}` |
| `desc` | 删除：注释即可 |
| `after` | 删除：`stages` 列表顺序即顺序 |
| `${image.base.tag}` | 改为字面量（与 base stage 的 `-t` 相同） |
