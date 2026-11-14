# materials sync 设计约束（已定稿部分）

> 目标：下载 / 解包 / marker 统一由 `cnb materials sync` 负责（旧 fetch.sh 已删除）。

## 一、已拍板的决策

| 主题 | 决策 |
|---|---|
| 校验 | **只做 sha256 + size**；签名、sha512 暂不做 |
| 并发 | 默认 **4** 个物料并行；单物料内串行 |
| 通配符 | 简单两档：`*` 不跨分隔符（`/ : @`），`**` 可跨 |
| 时间格式 | 统一 **UTC + RFC3339（Z 结尾）**，用 `internal/timefmt` |
| ref 解析 | 结构化 `internal/ref.Ref{ID,Version}`，只解析一次 |

## 二、源的选择顺序

```
环境变量镜像（GH_MIRROR / BP_GO_MIRROR …）
  → lock 的 [asset.platform.<p>] mirrors[]
  → lock 的 url（权威地址，check-updates 仍以它为准）
```

## 三、解包规则（lock 字段）

```toml
[asset.unpack]
kind    = "tgz"        # tgz | tar.gz | zip | bin
strip   = 1
include = ["docker/**"] # 只解压匹配项；空表示全部
mode    = "0755"        # 解出的可执行位（bin 必填）
```

特殊产物对照：

| 产物 | 配置要点 |
|---|---|
| docker | `include=["docker/docker"]`（只需要 CLI） |
| go SDK | `kind=tgz, strip=1` |
| fnm | `kind=zip, strip=0` |
| lifecycle | `kind=tgz, strip=1`，保留软链 `exporter -> lifecycle` |
| jq / yq | `kind=bin, mode=0755` |

## 四、marker（JSON）

```json
{
  "ref": "docker@27.0.3",
  "platform": "linux-amd64",
  "sha256": "…",
  "size": 73823487,
  "source_url": "…",
  "extractor": "tgz/v2",
  "files": ["docker/docker"],
  "synced_at": "2026-11-13T09:00:00Z"
}
```

用途：判断已同步、规则变更后自动失效、分析系统读取来源与时间。

## 五、命令

```bash
cnb materials sync [ids...]
cnb materials sync --offline   # 只校验
cnb materials sync --check     # 只校验
cnb materials sync --jobs 4
```

旧 `fetch.sh` 已删除，just 的 fetch 直接调用 `cnb materials sync`。

## 六、安全约束

- 拒绝绝对路径、`../` 逃逸、越界软链
- 先解到临时目录，成功后原子 rename
- 校验未通过时不覆盖已有产物

## 七、实现进度

| 步骤 | 状态 |
|---|---|
| HTTP 下载（重试 3 次、指数退避、MATERIALS_MIRROR 前缀） | ✅ |
| sha256 + size 校验，不通过不覆盖 | ✅ |
| 解包：tgz / zip / bin、strip、include 通配、软链、防路径逃逸 | ✅ |
| JSON marker（含 extractor 版本，规则变化自动失效） | ✅ |
| `cnb materials sync`（默认 4 并发，--offline / --check / --jobs） | ✅ |
| 兼容旧 `.sha256` 标记（产物齐全时补写新 marker） | ✅ |
| `script/fetch.sh` 收编为一行转发 | ⏳ 待做 |
| 契约测试（沿用 test/contract） | ⏳ 待做 |

lock 新增字段：

```toml
[asset.platform.linux-amd64]
url    = "..."
sha256 = "..."
size   = 73823487          # 新增（可选）
size   = 0                 # 未声明时不校验大小

[asset.unpack]
include = ["docker/docker"]  # 新增（可选，通配符）
```
