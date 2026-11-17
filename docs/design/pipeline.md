# 完整流水线

一份配置描述流程，两个脚本负责执行，物料只有一份。

```
┌─ 配置（唯一改动点）──────────────────────────────────────────────┐
│ assets.lock.toml            物料：版本 / URL / sha256 / 平台      │
│ config/projects/<p>.toml    流程：物料 / 镜像 / 命令 / 顺序       │
└──────────────────────────────────────────────────────────────────┘
                              │
        项目/build.sh ────────┴──────── just build / just plan
                              │
                    script/bin/cnb-linux-amd64 pipeline run <profile>
                              │
                 ┌────────────┴─────────────┐
                 │ cnb pipeline run：校验 + 计划 + 执行 │
                 └────────────┬─────────────┘
                              │ eval
        ┌─────────────────────┼──────────────────────┐
        ▼                     ▼                      ▼
  fetch_assets          image_build            run_command
  script/fetch.sh    script/lib/image-build.sh   bash -c
        │                     │                      │
        ▼                     ▼                      ▼
  bundle/<platform>/    podman|docker build     scripts/build.sh
  {archives,binaries}   (context = 仓库根)      + cnb pipeline repack
```

## 一次完整构建

```bash
./project/offline-paketo-bk/build.sh --dry-run    # 1. 看计划
./project/offline-paketo-bk/build.sh              # 2. 执行
```

展开后等价于：

```bash
# ① 物料：按 lock 下载 + 校验 + 解压（三态幂等，断了可重跑）
script/fetch.sh fetch lifecycle@0.21.13 fnm@1.39.0 … --platform linux-amd64

# ② 镜像：顺序来自 [[stage]]，上下文固定为仓库根
script/lib/image-build.sh <repo> <repo>/project/offline-paketo-bk/docker/Dockerfile.builder-base \
  offline-builder-base-bk:latest --build-arg BASE_IMAGE=…

# ③ 命令：打包（bksmartpacker 产出 SmartPackage，再经 cnb pipeline repack 规范化）
(cd <repo>/project/offline-paketo-bk && scripts/build.sh)
```

## 分工

| 层 | 位置 | 负责 | 不负责 |
|---|---|---|---|
| 事实 | `assets.lock.toml` | 物料版本 / URL / 哈希 | 流程 |
| 流程 | `config/projects/*.toml` | 用哪些物料、建哪些镜像、顺序 | 具体命令实现 |
| 规划 | `cnb profile render` | 校验 + 生成 shell 计划 | 执行 |
| 执行 | `cnb pipeline run` | 校验、生成计划、调用原语执行（slog 日志） | 决策 |
| 原语 | `fetch.sh` / `image-build.sh` | 下载解压 / 单镜像构建 | 顺序 |
| 资产 | `<proj>/docker/`、`<proj>/scripts/` | Dockerfile、buildpack、打包脚本 | 不被统一 |

## 改东西去哪

| 想改 | 改这里 |
|---|---|
| 升物料版本 | `assets.lock.toml` |
| 某项目锁版本 | profile 的 `[assets.<id>] version` |
| 换基础镜像 / tag | profile 的 `[images.*]` `base` / `tag` |
| 加一个构建步骤 | `[commands.<名>]` + 一条 `[[stage]]` |
| 调顺序 | 移动 `[[stage]]` |
| 换平台 | `--platform` 或 `[project] platform` |

## 校验

```bash
just check                                # lock / 模板 / shell / golang / 单测
just verify                               # 项目内：只校验本流程用到的物料
just --justfile justfile verify            # 仓库根：校验 lock 里的全部物料
cnb profile validate config/projects/*.toml  # 流程本身
cnb profile show config/projects/<p>.toml    # 分组查看
```

流程规则（`validate` 强制）：

1. `[[stage]]` 顺序即执行顺序，每步只引用一个 `image` 或 `command`
2. 定义了却没进流程的镜像/命令会报错
3. `${image.<n>.tag}` 只能引用流程中更早出现的镜像

## 执行体验

```bash
./build.sh                 # 先打印计划摘要，再按 [i/N] 推进，每步计时
./build.sh --dry-run       # 只看计划（text）
./build.sh --only builder  # 只跑某一步，其余标注 skipped
```

失败时会给出可直接重跑的命令：`cnb pipeline run <cfg> --only <stage>`。

## 静态检查

```bash
cnb pipeline dockerfile check config/projects/<p>.toml   # COPY 源路径是否存在
```

已接入 `script/ci.sh`，能拦住"改了目录/改名后 Dockerfile 失效"这类问题。

## 尚未完成

- `package` 阶段指向 `scripts/build.sh`，该目录不入库，新克隆仓库需自备
- Dockerfile 模板化**已决定不做**：只保留 `dockerfile check` 作为路径守卫
- 物料哈希多为空（`verify` 会提示未固定），对外分发前应改为上游官方值
