# 如何维护 assets.lock.toml

面向“要加一个新工具 / 升一个版本”的人。先看结论，再看细节。

## 一·0、物料存放位置由 lock 声明

```toml
[storage]
root   = "bundle"      # 相对 lock 文件所在目录；也可写绝对路径
layout = "arch-first"  # arch-first | kind-first
```

- `cnb` 一律按 `[storage].root` 解析物料目录，不再"猜"
- 项目配置里的 `[docker].bundle_dir` 若填写，必须与之相同（validate 会报错）
- 改存放位置只改这一处，所有命令（sync/verify/publish/plan）自动跟随

## 一、字段归属（谁写）

| 字段 | 谁写 | 说明 |
|---|---|---|
| `id` | **人** | 物料标识，仓库内唯一 |
| `version` | **人** | 上游版本号（升版本只改这里） |
| `default` | 人 | 同 id 多版本时选哪个 |
| `[asset.info]` | 人 | 名称/主页/许可证，**不参与构建** |
| `[asset.unpack] kind/strip/dir/expect` | **人** | 解包规则；`dir` 决定落盘位置 |
| `[asset.unpack] include` | 人（可选） | 只解压匹配路径，支持 `*` / `**` |
| `[asset.platform.*] kind` | 人 | `http`（默认）或 `local` |
| `[asset.platform.*] url` | **人** | 下载地址，支持唯一的 `{version}` 占位 |
| `[asset.platform.*] mirrors` | 人（可选） | 回退源，仅传输/HTTP 错误时尝试 |
| `[asset.platform.*] source` | 人（local） | 本地构建物的仓库相对路径 |
| `[asset.platform.*] recipe` | 人（可选） | 本地构建物的复现命令 |
| `[asset.platform.*] sha256` | **自动** | http：`materials sync` 回填；local：`materials publish` 回填 |
| `[asset.platform.*] size` | **自动** | 字节数；`-1` 表示未知并跳过校验 |
| `bundle/` 下的文件、`.meta/*.sync.json` | **自动** | 不要手动改 |

原则：**地址与规则人写，哈希与落盘状态机器写。**

## 二、加一个新工具

1. 在 `assets.lock.toml` 末尾追加：

```toml
[[asset]]
id = "mytool"
version = "1.2.3"
default = true

  [asset.info]
  name     = "My Tool"
  homepage = "https://example.com/mytool"
  license  = "MIT"

  [asset.unpack]
  kind   = "tgz"          # tgz | tar.gz | zip | bin
  strip  = 1              # 去掉顶层目录
  dir    = "mytool/{version}"
  expect = ["mytool"]     # 相对 binaries/ 的产物路径

  [asset.platform.linux-amd64]
  url    = "https://example.com/mytool-{version}-linux-amd64.tar.gz"
  sha256 = ""
```

2. 拉取（会自动回填 sha256 / size）：

```bash
just fetch mytool
# 或
script/bin/cnb-linux-amd64 materials sync mytool
```

3. 校验：

```bash
script/bin/cnb-linux-amd64 materials validate
script/bin/cnb-linux-amd64 materials verify mytool
```

4. 在项目的 `config/projects/*.toml` 里引用并注入版本：

```toml
[assets.mytool]
version = "1.2.3"

[images.builder]
inject_versions = ["...", "mytool"]
```

## 三、升级版本

只改两处，然后重新拉取：

```diff
[[asset]]
-id = "mytool"        # 不变
-version = "1.2.3"
+version = "1.2.4"
```

```bash
just fetch mytool     # 旧哈希不匹配 → 重新下载并回填
```

如果上游改了目录结构（比如多了层 `bin/`），只需改 `strip` / `expect`，
`materials sync` 会因 `extractor` 变化自动重解。

## 四、本地构建物（如 cnb）

```toml
[[asset]]
id = "cnb"
version = "3.0.0"

  [asset.platform.linux-amd64]
  kind   = "local"
  source = "script/bin/cnb-linux-amd64"
  sha256 = ""

  [asset.platform.linux-arm64]
  kind   = "local"
  source = "script/bin/cnb-linux-arm64"
  sha256 = ""

  [asset.unpack]
  kind   = "bin"
  dir    = "cnb/{version}"
  expect = ["cnb"]
```

- `source` 人写，`sha256` 由 `cnb materials publish cnb --platform <p>` 自动回填
- `just tools` 已内置发布步骤

## 五、常见错误

| 现象 | 原因 |
|---|---|
| `unpack.kind 必须是 tgz/tar.gz/zip/bin` | kind 拼错 |
| `缺少 meta.platforms 声明的平台` | 物料没有覆盖所有平台 |
| `校验后缺少预期产物` | `expect` 与 `dir`/`strip` 不匹配 |
| 两个版本互相覆盖 | 不同版本必须用不同的 `dir`（含 `{version}`） |
| `sha256 not pinned` | 还没跑过 `materials sync`（或为 local 未 publish） |

## 六、命令速查

```bash
cnb materials list                      # 清单
cnb materials get <id> <platform>       # 输出 shell 变量
cnb materials sync [ids...]             # 下载 + 校验 + 解包 + marker
cnb materials sync --check              # 只校验
cnb materials verify [ids...]           # 校验落盘状态
cnb materials pin [ids...]              # 反推 sha256（只打印）
cnb materials publish <id>              # 发布本地构建物
cnb materials lock set-sha <id> <platform> <sha>
```
