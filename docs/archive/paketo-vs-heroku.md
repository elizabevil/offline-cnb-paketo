> **归档**：Heroku 组件已从仓库移除，本文仅作上游对比参考。

# Paketo 与 Heroku CNB 对比

> 范围：**Paketo** 与 **Heroku Cloud Native Buildpacks**。

两者都遵循 CNB 规范（buildpack API / platform API、`launch.toml`、layers 目录），
差异在组织哲学与工程取舍。

## 一、buildpack 粒度

| 维度 | Paketo | Heroku |
|---|---|---|
| 组织方式 | 小步组合：一个语言拆成多个 buildpack | 单语言单体：一个 buildpack 包办 |
| Go 示例 | `go-dist`（提供 Go）+ `go-build`（编译）+ `procfile`（进程） | `buildpacks-go`（检测、编译、依赖、进程全包） |
| Node 示例 | `node-engine` + `npm-install` + `npm-start` | `buildpacks-nodejs` 单体 |
| order 组 | 通常 3~5 个 buildpack 一组 | 通常 1 个 buildpack 一组 |

影响：

- Paketo 的每一层职责单一，替换/扩展一个环节只需替换一个 buildpack
- Heroku 的行为集中在一个 buildpack 内，改行为等于改 buildpack 本身

## 二、进程定义与 Procfile

| 维度 | Paketo | Heroku |
|---|---|---|
| 默认进程 | 语言 buildpack 写 `launch.toml` 的默认 `web` | 语言 buildpack 读取 `Procfile` |
| Procfile 处理 | 独立的 `procfile` buildpack | 语言 buildpack 内部实现 |
| 优先级 | order 组内后执行者覆盖先写者 | 由语言 buildpack 逻辑决定 |

影响：

- Paketo 下「语言默认进程」和「Procfile 进程」是两个来源，需要显式安排顺序
- Heroku 下 Procfile 是唯一入口，行为更直观，但无法只替换进程逻辑

## 三、基础镜像与栈

| 维度 | Paketo | Heroku |
|---|---|---|
| builder | `paketobuildpacks/builder-jammy-*`（full / base / tiny） | `heroku/builder:22`、`:24` |
| stack / base | Ubuntu 22.04 / 24.04；tiny 为 distroless 风格 | `heroku-22`、`heroku-24`（Ubuntu + Heroku runtime） |
| 运行时基座 | Paketo 维护 run image | Heroku 维护 run image |
| 旧栈状态 | bionic 已淘汰 | heroku-18 已 EOL |

影响：

- 栈决定 run image、libc（glibc/alpine）、可用系统包
- 离线方案必须固定 builder 与 run image 的版本，避免上游滚动更新导致不可复现

## 四、依赖与供应链

| 维度 | Paketo | Heroku |
|---|---|---|
| 依赖来源 | 官方 dependency mirror + buildpack registry | S3 bucket + inventory 文件 |
| SBOM | 每层生成 CycloneDX / SPDX，粒度细 | 有 SBOM，粒度较粗 |
| 更新节奏 | 每个 buildpack 独立发版 | builder 镜像每日重建，聚合各语言 |

影响：

- Paketo 的产物更容易做逐层审计
- Heroku 的产物版本以 builder 镜像为单位，升级是「整体换镜像」

## 五、离线与定制

| 维度 | Paketo | Heroku |
|---|---|---|
| 离线构建 | builder 镜像 + bundle（buildpack 包） | builder 镜像自包含，最省事 |
| 自定义 buildpack | 按层组合，扩展成本低 | 需修改语言 buildpack 本身 |
| 缓存粒度 | 每个 buildpack 的 layer 独立 | 集中在语言 buildpack 内部 |

## 六、对我们离线方案的影响

### 6.1 组件结构

| 选择 | 结果 |
|---|---|
| Paketo 风格 | 自建 `go-dist` / `go-build` / `procfile` 等小组件，order 里组合；可读性与可替换性最好 |
| Heroku 风格 | 直接复用单体 buildpack；改动少，但内部行为不可控 |

### 6.2 进程定义

- 走 Paketo：必须在 order 中显式安排 `procfile` 的位置，否则语言 buildpack 的默认进程会覆盖 Procfile
- 走 Heroku：进程来自 Procfile，无需额外编排
- 两种路线都要保证「进程命令在镜像内可执行」，与导出方式无关

### 6.3 离线物料

| 物料 | Paketo 路线 | Heroku 路线 |
|---|---|---|
| builder | 需要自建（base + lifecycle + buildpacks） | 可直接使用官方 builder 镜像 |
| run image | 与 stack 绑定，需要离线保存 | 与 stack 绑定，需要离线保存 |
| buildpack | 多个小组件，逐个固定版本 | 少量单体，整体固定版本 |
| 校验 | 每个组件独立 sha256 | 以 builder 镜像 digest 为准 |

### 6.4 导出与交付

- 两种路线的导出方式相同：`exporter`（daemon 或 layout）+ 归档
- 差异只在镜像内容：Paketo 的层更碎，Heroku 的层更集中
- 离线交付建议固定为 OCI layout，避免依赖容器运行时的 docker 兼容层

## 七、选型建议

| 目标 | 建议 |
|---|---|
| 学习 CNB、可控可组合、便于替换组件 | Paketo |
| 行为稳定、少维护、接近官方一把梭 | Heroku |
| 需要精细 SBOM / 逐层审计 | Paketo |
| 需要最少改动接入现有平台 | Heroku |

## 八、参考

- Paketo：https://paketo.io / https://github.com/paketo-buildpacks
- Heroku CNB：https://github.com/heroku/buildpacks / https://github.com/heroku/cnb-builder-images
- CNB 规范：https://github.com/buildpacks/spec
