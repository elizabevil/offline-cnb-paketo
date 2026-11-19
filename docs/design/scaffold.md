# 脚手架：构建工具的生成流程

`cnb init` 负责把「一套构建流程」从零生成出来。它不是代码生成器，而是**模板渲染器**：
模板在代码里（内嵌），输出是可读可改的配置文件与 Dockerfile。

## 1. 生成什么

```
project/<name>/
├── pipeline.toml            # 流水线：阶段、占位符、静态检查
├── assets.toml              # 物料清单：第三方制品（版本 / URL / 解包规则）
├── justfile                 # 任务入口：check / lock / sync / plan / build
├── README.md                # 项目说明：布局与常用命令
└── docker/
    ├── base.dockerfile      # 基础镜像：所有阶段的地基
    └── builder.dockerfile   # 构建镜像：lifecycle + buildpacks
```

四者是一套，缺一不可：

| 文件 | 谁读它 | 缺失后果 |
|---|---|---|
| `pipeline.toml` | `cnb pipeline plan/run` | 无法构建 |
| `assets.toml` | `cnb materials lock/sync` | `${spec.assets.*}` 解析失败 |
| `base.dockerfile` | 第一条 stage | 构建失败 |
| `builder.dockerfile` | 第二条 stage | 无法产出 builder 镜像 |

## 2. 模板注册表

模板清单是**数据**，定义在 `internal/scaffold/scaffold.go` 的 `Templates()`：

| id | 输出 | 渲染来源 |
|---|---|---|
| `config` | `<dir>/pipeline.toml` | `internal/spec/profile` 模板 |
| `assets` | `<dir>/assets.toml` | `internal/spec/manifest` 模板 |
| `justfile` | `<dir>/justfile` | `internal/scaffold/templates` |
| `readme` | `<dir>/README.md` | 同上 |
| `dockerfile-base` | `<dir>/docker/base.dockerfile` | 同上 |
| `dockerfile-builder` | `<dir>/docker/builder.dockerfile` | 同上 |

新增一种生成文件 = 在 `Templates()` 加一条；删除 = 删一条。命令层不含任何模板知识。

## 3. 选择与覆盖

```bash
cnb init demo --list                      # 看注册表
cnb init demo                             # 全部生成
cnb init demo --only dockerfile-base      # 只生成指定项
cnb init demo --skip assets               # 跳过指定项
cnb init demo --force                     # 覆盖已存在文件
```

默认**不覆盖**已存在文件，避免把用户改过的配置冲掉。

## 4. 参数

| 参数 | 影响 |
|---|---|
| `--dir` | 项目目录（默认 `project/<name>`） |
| `--platform` | pipeline 的目标平台（单值） |
| `--platforms` | assets 声明的平台（多值，默认 amd64+arm64） |
| `--base-image` | `[spec.project.base]` 的镜像 |
| `--container` | stage 首项容器命令（默认 `podman`，可用 `CNB_CONTAINER`） |

`platform` 与 `platforms` 是两个概念：前者是**本次构建目标**，后者是**物料覆盖范围**。

## 5. 为什么模板放代码里

- 模板与解析器同仓库，改动时 CI 会渲染并校验（`just check`）
- 不再维护 `assets.toml.example` 这类"文档即模板"的文件，避免两份漂移
- 渲染结果由 `profile validate` / `materials validate` 兜底

## 6. 与其他命令的关系

```
cnb init               生成（本文件）
   ↓
cnb materials lock     下载物料并写哈希
   ↓
cnb pipeline plan      展开占位符，打印最终 argv
   ↓
cnb pipeline run       执行
   ↓
cnb lint               静态检查（构建参数、Dockerfile COPY 源）
```
