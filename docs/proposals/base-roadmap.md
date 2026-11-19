# 基座：待拍板决策与演进路线

> 状态：**提案**。部分决策已被后续实现取代，保留作为取舍记录。
> 当前约定见 [`docs/design/base.md`](../design/base.md)。

## 一、待拍板决策

| # | 决策 | 我的建议 | 影响面 |
|---|---|---|---|
| 1 | 根 `bundle/` 与子项目 `bundle/` 的关系 | 根为唯一源，子项目通过 `BUNDLE_DIR` 或 symlink 指向；`scripts/` 逐步迁到根 `script/` | 三个子项目的 `.just` / Dockerfile 路径 |
| 2 | stack id 用什么 | 废弃 `io.buildpacks.stacks.bionic`，改用 target 声明（Buildpack API 0.12 的 `[[targets]]`）或 `io.buildpacks.stacks.jammy` | builder/run 镜像标签、`pack build` 兼容性 |
| 3 | SDK 真离线方案 | 预置 `bundle/<platform>/sdk/{node,python,go}` + 本地 index，去掉 `fnm install`/`uv venv --python` 的联网行为 | Node/Python buildpack 的 build 阶段 |
| 4 | 是否引入 CI | 先加一条 `just check && just verify` 的 pre-commit/CI 门禁 | 后续所有阶段的回归保护 |

---

## 二、演进路线

```
P0  物料基座（已完成）
    ├─ assets.lock.toml
    ├─ script/{justfile, ci.sh} + tools/cnb (Go: 全部逻辑)
    ├─ 根 justfile（任意目录可用）
    └─ bundle/ 骨架 + 三态幂等 + 校验

P1  Buildpack 契约层
    ├─ script/lib/cnb.sh：layer.toml([types]) / launch.toml / env/PATH.prepend 统一生成
    └─ 修掉 node/python 的 `.launch` 与 go 的 command 数组问题

P2  镜像层
    ├─ builder / run 双镜像，stack 一致
    ├─ entrypoint 可移植（bash / 无 mountpoint 依赖 / skip 组合自洽）
    └─ builder 元数据标签由 lock 生成，杜绝手写版本号

P3  验证层
    ├─ 4 应用 × (detect / build / run) 矩阵
    ├─ SBOM 产出
    └─ CI 门禁
```
