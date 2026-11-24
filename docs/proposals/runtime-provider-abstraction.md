# 运行时提供者抽象（uv / fnm 可替换方案）

> 状态：**提案**。不修改代码，先定义接口与迁移路径。

## 一、为什么需要

当前 Python/Node 的版本管理直接依赖两个工具：

| 语言 | 工具 | 使用位置 |
|---|---|---|
| Python | `uv` | `cpython/bin/build`、`pip-install/bin/build` |
| Node | `fnm` | `node-engine/bin/build` |

它们被复制进 builder：

```dockerfile
COPY --from=bundle .../fnm  /usr/local/bin/fnm
COPY --from=bundle .../uv   /usr/local/bin/uv
COPY --from=bundle .../uvx  /usr/local/bin/uvx
```

如果以后换成 `mise`、`asdf`、`pyenv`、`nvm` 或直接用官方 tarball，改动会散落在：

1. builder Dockerfile 的 COPY
2. buildpack 脚本里的命令行
3. 缓存目录与环境变量
4. 物料版本与下载地址
5. 文档与变量登记表

## 二、当前耦合点

| 耦合点 | 具体内容 |
|---|---|
| 二进制名 | `uv`、`uvx`、`fnm` 写死在脚本 |
| 子命令 | `uv python install`、`uv venv`、`uv pip install`、`fnm install` |
| 环境变量 | `UV_PYTHON_INSTALL_MIRROR`、`UV_PYTHON_INSTALL_DIR`、`UV_INDEX_URL`、`FNM_PATH`、`--fnm-dir` |
| 缓存布局 | Python 在 `/layers/cache/python/install`，Node 在 `/layers/cache` |
| 版本声明 | Python 读 `runtime.txt`，Node 用 `BP_NODE_VERSION` |
| 物料 | `assets.toml` 中 uv/fnm 的版本、URL、哈希 |
| 变量登记 | `buildpacks/paketo/variables.toml` |

## 三、目标

1. buildpack 不直接调用具体工具
2. 换工具只改适配层与 builder 物料
3. 版本声明、镜像源、缓存语义保持稳定
4. 允许“工具替换”和“运行时来源替换”分开做

## 四、抽象接口

在 buildpacks 中引入共享库：

```
buildpacks/paketo/lib/runtime.sh
```

定义四组函数：

```sh
# 解析版本：BP_<LANG>_VERSION > 语言版本文件 > 默认
runtime_resolve_version <lang> <app_dir>

# 安装/定位运行时，输出运行时根目录
runtime_install <lang> <version> <install_dir> <mirror>

# 导出运行时所需环境变量（PATH 等）
runtime_env <lang> <root>

# 安装依赖
runtime_install_deps <lang> <app_dir> <venv_or_node_modules> <index_config>
```

buildpack 只调用这些函数，不出现 `uv` / `fnm` 字样。

## 五、提供者实现

| 提供者 | Python | Node | 说明 |
|---|---|---|---|
| `uv` | `uv python install` + `uv venv/pip` | — | 当前 |
| `fnm` | — | `fnm install` | 当前 |
| `tarball` | 直接下载/解压 python-build-standalone | 官方 node 二进制 | 完全离线友好 |
| `mise` | `mise install python@...` | `mise install node@...` | 统一多语言 |
| `system` | 使用镜像内已有解释器 | 使用镜像内已有 node | 最小镜像 |

选择方式：

```dotenv
BP_RUNTIME_PROVIDER_PYTHON=uv
BP_RUNTIME_PROVIDER_NODE=fnm
```

默认保持当前实现，替换时只改这一行。

## 六、环境变量规范化

工具专用变量不应泄漏到项目配置。建议：

| 规范变量 | 适配为 |
|---|---|
| `BP_PYTHON_VERSION` | 各提供者的版本参数 |
| `BP_PYTHON_MIRROR` | `UV_PYTHON_INSTALL_MIRROR` / tarball 前缀 |
| `BP_PYTHON_INDEX_URL` | `UV_INDEX_URL` / pip `--index-url` |
| `BP_PYTHON_FIND_LINKS` | `--find-links` |
| `BP_PYTHON_NO_INDEX` | `--no-index` |
| `BP_NODE_VERSION` | fnm/mise 版本 |
| `BP_NODE_MIRROR` | node 二进制前缀 |
| `BP_NODE_REGISTRY` | npm registry |

旧变量（`UV_*`、`FNm_*`）作为别名保留一个版本周期。

## 七、缓存与标记

缓存按“提供者 + 版本”隔离：

```
/layers/cache/python/<provider>/<version>/
/layers/cache/node/<provider>/<version>/
```

marker 记录：

```json
{ "provider": "uv", "version": "3.11.10", "sha": "..." }
```

换提供者时不会误用旧缓存。

## 八、迁移步骤

| 步骤 | 内容 | 影响 |
|---|---|---|
| 1 | 新增 `lib/runtime.sh` 与 uv/fnm 实现 | 无行为变化 |
| 2 | cpython/pip-install 改调接口 | 无行为变化 |
| 3 | node-engine 改调接口 | 无行为变化 |
| 4 | 增加 `tarball` 提供者，用于离线 | 可选 |
| 5 | 文档/变量登记表切换到 `BP_*` 规范名 | 兼容旧名 |
| 6 | 删除 uv/fnm 专用变量 | 破坏性，下一大版本 |

## 九、替换时需要改什么

| 场景 | 需要改 |
|---|---|
| 只换工具（uv → mise） | 提供者实现 + builder 物料（mise 二进制） |
| 换运行时来源（PBS → 系统 python） | 提供者实现；版本文件语义不变 |
| 只做离线 | 增加 `tarball` 提供者或本地文件镜像，不改 buildpack |
| 增加语言 | 新 buildpack + 一个提供者实现 |

## 十、验证

1. 提供者一致性测试：同一组版本声明，各提供者输出相同版本
2. fake 提供者：只记录调用参数，验证 buildpack 不直接调工具
3. 构建矩阵：uv/fnm、tarball 各跑一次 Python/Node 应用
4. 缓存切换：更换 provider 后不复用旧缓存

## 十一、待定

- 是否统一用 `mise` 替代 uv+fnm
- 运行时 tarball 是否纳入物料
- 旧 `UV_*` 变量保留多久
- `lib/runtime.sh` 是否跨 buildpack 共享（需要 builder 内置）

## 十二、共享库的位置与打包

共享库不是 buildpack，不能依赖生命周期调度，必须在 builder 构建时进入镜像。

### 仓库位置

```
buildpacks/paketo/_lib/
├── runtime.sh          # 入口：选择 provider
└── providers/
    ├── uv.sh
    ├── fnm.sh
    └── tarball.sh
```

用 `_lib` 前缀，避免被 `cnb buildpack lint buildpacks/*/*` 当成 buildpack。

### 镜像位置

```
/cnb/lib/paketo/runtime.sh
/cnb/lib/paketo/providers/*.sh
```

两个 builder Dockerfile 都需要：

```dockerfile
COPY --from=buildpacks /_lib/ /cnb/lib/paketo/
```

### buildpack 调用

```sh
. /cnb/lib/paketo/runtime.sh

runtime_provider python
runtime_install python "$version" "$install_root"
```

buildpack 不直接 source provider，也不出现 `uv` / `fnm` 字样。

### 需要同步调整

| 项 | 调整 |
|---|---|
| buildpack lint glob | 跳过 `buildpacks/paketo/_lib` |
| `builder.dockerfile` / `builder-build.dockerfile` | 复制 `_lib` |
| `variables.toml` | 登记 `BP_RUNTIME_PROVIDER_*` |
| 文档 | 记录 `/cnb/lib/paketo` 路径 |

### 流程位置

```
仓库 _lib
  → builder Dockerfile COPY
  → 镜像 /cnb/lib/paketo
  → lifecycle build
  → buildpack source 后调用 provider
```

## 十三、不做

- 不引入运行时服务
- 不在应用源码里写提供者配置
- 不在 buildpack 中直接判断工具名
