# 脚手架：从参考项目生成

`cnb init` 不再渲染手写模板，而是**复制一个能跑的项目**再改写名称。
模板会与真实项目漂移，参考项目不会。

## 1. 用法

```bash
cnb init demo                                  # 参考 project/offline-paketo
cnb init demo --from project/offline-paketo    # 显式指定参考项目
cnb init demo --dir project/x                  # 输出目录
cnb init demo --only docker                   # 只复制 docker/ 下文件
cnb init demo --only docker --only assets.toml # 多个 --only 累积
cnb init demo --skip fixtures --skip docs      # 跳过指定子路径
cnb init demo --skip assets.toml               # 排除 assets.toml
cnb init demo --force                          # 覆盖已存在文件
```

## 2. 生成过程

1. 遍历参考项目目录，逐个文件复制
2. 文本文件里把 `offline-paketo` 替换为新名字，`project/offline-paketo` 替换为新目录
3. 二进制文件（含 NUL 字节）原样复制
4. 仓库根的 `assets.toml` 作为普通候选文件参与过滤，默认复制到 `<dir>/assets.toml`

过滤规则只有一条：pattern 等于路径、是父目录，或匹配 glob（`filepath.Match`）。
`--only` 与 `--skip` 对参考项目文件与 `assets.toml` 一视同仁——
因此 `--only docker` 不会带 assets，需要时显式再加一个 `--only assets.toml`。

## 3. 项目目录是自包含的

```
project/<name>/
├── pipeline.toml      # 流水线（配置就在项目内）
├── assets.toml        # 物料清单
├── justfile           # 任务入口
├── docker/            # Dockerfile 与构建上下文
└── docs/ fixtures/    # 参考项目带走的内容，可用 --skip 排除
```

`config/projects/` 已删除：配置放在项目目录内，整个目录可复制、可移动。

## 4. 校验

```bash
cnb pipeline profile validate project/demo/pipeline.toml
cnb materials validate --file project/demo/assets.toml
```

`just check` 会遍历 `project/*/pipeline.toml` 做同样校验。

## 5. 为什么不再维护模板

| 手写模板 | 从参考项目复制 |
|---|---|
| 需要同步每次结构调整 | 结构天然一致 |
| 出现过命名、环境变量、示例三类漂移 | 不会漂移 |
| 无法验证"生成的项目能否跑" | 源头就是能跑的项目 |

代价：会把参考项目里不想要的内容（fixtures、docs）一起带过来，用 `--skip` 处理。
