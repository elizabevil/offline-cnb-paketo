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

### 5.2 spec/profile —— `config/projects/<name>.toml`

```toml
schema = 1

[project]
name     = "offline-paketo-bk"
dir      = "project/offline-paketo-bk"
platform = "linux-amd64"

[build]
aliases = { "bk-buildpack-go" = "paketo-buildpacks/go" }   # 平台别名由项目自带

[images.base]
dockerfile = "docker/base.dockerfile"
tag        = "offline-builder-base-bk:latest"
args       = { BASE_IMAGE = "debian:trixie-slim" }

[commands.package]
run     = "project/offline-paketo-bk/scripts/build.sh"
workdir = "project/offline-paketo-bk"

[[stage]]
image = "base"

[[stage]]
command = "package"
```

职责：

- 结构体：`File` / `Project` / `Build` / `Image` / `Command` / `Stage`
- `spec/profile` 只解析与校验；**变量展开**（`${platform}`、`${arch}`）属于 `internal/plan`
- 校验：名称非空、镜像引用存在、阶段顺序（引用的镜像必须先出现）、别名 key 合法
- 删除 `[docker].bundle_dir`：物料位置只由 `manifest.[storage]` 决定

### 5.3 spec/lifecycle —— 6 个 lifecycle 协议

统一在一个包，按文件名分文件：

```
spec/lifecycle/
├── order.go        order.toml（读写）
├── buildpack.go    buildpack.toml
├── plan.go         plan.toml
├── analyzed.go     analyzed.toml
├── launch.go       launch.toml
└── layer.go        layer.toml
```

`order.toml` 示例（`Parse` 与 `Filter` 共用同一结构体）：

```toml
[[order]]
  [[order.group]]
  id      = "paketo-buildpacks/go"
  version = "4.2.1"

  [[order.group]]
  id      = "paketo-buildpacks/go-build"
  version = "2.3.0"
```

`analyzed.toml` 示例（只读）：

```toml
[run-image]
  reference = "index.docker.io/library/debian:trixie-slim"
  top-layer = "sha256:0e3f…"

[[previous-image]]
  ...
```

`launch.toml` 示例（只读，供检查/打包使用）：

```toml
[[processes]]
  type    = "web"
  command = ["/app/bin/start-web.sh"]
  default = true
```

职责：

- 事务：`lifecycle` 包负责写 `order`（过滤后），其余只读
- `checks` 不再用正则解析 `order.toml` 与 `buildpack.toml`，改为调用本包
- 删除 `internal/lifecycle/order.go` 的结构体定义（迁入本包）

### 5.4 spec/platform —— 平台契约

```go
package platform

type Env struct {
    SourceURL      string   // SOURCE_GET_URL（仅 file://）
    Required       []string // REQUIRED_BUILDPACKS
    OutputImage    string   // OUTPUT_IMAGE
    RunImage       string   // CNB_RUN_IMAGE
    CacheImage     string   // CACHE_IMAGE
    AppDir         string   // CNB_APP_DIR
    LayersDir      string   // CNB_LAYERS_DIR
    PlatformDir    string   // CNB_PLATFORM_DIR
    PlatformAPI    string   // CNB_PLATFORM_API
    UseDaemon      bool     // USE_DOCKER_DAEMON
    ExportMode     ExportMode // CNB_EXPORT_MODE
}

func ParseEnv(getenv func(string) string) (Env, []warn.Warning, error)
func (e Env) Validate() []error
```

规则：

- **唯一**读取环境变量的地方；业务包只接收 `Env`
- 参数表驱动：`--export` 与 `CNB_EXPORT_MODE` 的映射写在同一张表里，避免双写
- `SOURCE_GET_URL` 的 `file://` 解析也在此，`lifecycle/prepare` 不再自己拆字符串
- BK 的 buildpack 别名不进本包（放 profile 的 `[build].aliases`）

### 5.5 spec/imagefmt —— 镜像归档格式

```go
package imagefmt

// docker-archive（manifest.json + <diffid>/layer.tar）
func ReadDockerArchive(path string) (*Image, error)
func WriteDockerArchive(img *Image, path string) error

// OCI layout（oci-layout + index.json + blobs/sha256/*）
func ReadOCILayout(dir string) (*Image, error)
func WriteOCILayout(img *Image, dir string) error

// 统一模型
type Image struct {
    RepoTags []string
    OS, Arch string
    Config   Config
    Layers   []Layer   // 顺序即层顺序
}
```

职责：

- `repack` 只做“格式转换策略”，读写调用本包
- `archive` 只保留 tar/zip 解包原语，不再写 docker-archive
- 格式判定（gzip 魔数、OCI vs docker-archive）集中在本包的探测函数
- 删除 `internal/archive/applayer.go`（BK 已改走 exporter + image save）

### 5.6 BK 私有协议（不进 cnb）

```
script/tools/bksmartpacker/
└── pkg/spec/bk/
    ├── app_desc.go     app_desc.yaml（modules / sourceDir / processes）
    ├── artifact.go     artifact.json
    └── smartpkg.go     s-mart 包结构（.Version / <module>.tgz / logo）
```

`.Version` 的取值 `cnb-image-layers` 只允许出现在 `smartpkg.go` 的常量里。

## 六、落盘后的目录

```
script/tools/cnb/internal/
├── spec/
│   ├── manifest/
│   ├── profile/
│   ├── lifecycle/
│   ├── platform/
│   └── imagefmt/
├── materials/     # 业务：下载/校验/解包（消费 manifest）
├── plan/          # 业务：变量展开、阶段编排（消费 profile + manifest）
├── lifecycle/     # 业务：执行四阶段（消费 platform + lifecycle）
├── checks/        # 业务：静态检查（消费 lifecycle + profile）
├── repack/        # 业务：格式转换（消费 imagefmt）
└── …              # archive 只留 tar/zip 原语
```

## 七、删除清单（不做兼容）

| 删除项 | 原因 |
|---|---|
| `materials` 的 `.sha256` 旧标记与 `MarkerUnpinned` 分支 | 双协议并存 |
| `checks/buildpack.go` 的 `orderRe` / `groupRe` 正则 | 改调 `spec/lifecycle` |
| `internal/lifecycle/order.go` 的结构体定义 | 迁入 `spec/lifecycle` |
| `internal/archive/applayer.go`（docker-archive 写出） | 保留 `spec/imagefmt` 一处实现 |
| `repack` 内部的 manifest / index 结构体 | 改用 `spec/imagefmt` |
| profile 的 `[docker].bundle_dir` | 物料位置只由 `manifest.[storage]` 决定 |
| `lifecycle_env.go` 的 `defaultAliases` | 移到 profile `[build].aliases` |
| `cnb materials get` 的 shell 赋值格式 | 只保留结构化输出（旧消费者已删除） |

## 八、验收方式

| 层级 | 测试 |
|---|---|
| 解析 | `test/contract/spec/*.golden` 金样本，解析后字段快照 |
| 编码 | 解析 → 编码 → 再解析，结构相等（round-trip） |
| 未知字段 | 未知键必须进入 `Warn`，并由 `cnb config doctor` 展示 |
| 单一解析器 | 静态检查：`toml.Unmarshal` 只允许出现在 `internal/spec/**` |
| 边界 | 静态检查：`internal/**` 中不得出现 `BKS_`、`app_desc`、`.Version` |

新增静态检查落到 `just check`（grep 规则 + go vet）。

## 九、迁移顺序

| 阶段 | 内容 | 验收 |
|---|---|---|
| 1 | `spec/lifecycle`：迁 `order`、`buildpack`，`checks` 改用 | `buildpack lint` 与 `lifecycle run` 都通过同一结构体 |
| 2 | `spec/manifest`：迁 lock 结构与校验 | `materials validate` 金样本一致 |
| 3 | `spec/platform`：合并 env 解析与参数表 | `lifecycle run` 只用 `platform.Env` |
| 4 | `spec/profile` + `plan` 拆包 | profile 不再做变量展开 |
| 5 | `spec/imagefmt`：合并 repack / archive / smartpkg 读取 | `repack` 与 `package inspect` 结果一致 |
| 6 | 删除清单逐项清理 | `just check` 含新增边界检查 |

每阶段独立提交，附带金样本与边界检查；前一阶段未绿不进下一阶段。
