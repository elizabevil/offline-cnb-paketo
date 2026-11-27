# 镜像包体积优化需求（专项）

> 状态：**D1 union 已端到端验证通过（上传 + 四模块部署 + Pod 运行）**；D6 多镜像方案被平台加载器否决。
> 目标是把 SOPS 测试应用的 s-mart 包压到 200 MB 以内；
> 后续 Go/Node 等多模块应用复用同一方案。

## 0. 术语（避免歧义）

| 术语 | 定义 |
|---|---|
| 应用 app | 一个 `app_desc.yaml` 描述的完整系统，例如 SOPS |
| 模块 module | `app_desc.modules[]` 的一项，例如 `default`、`api`；模块是构建单元，对应一个镜像、一个 `<module>.tgz` |
| 进程 process | `modules[].spec.processes[]` 的一项，例如 `web`、`beat`；进程是镜像内的启动项，不是独立镜像 |
| 镜像 image | 一个模块构建出的 OCI 镜像，由若干 layer 组成 |
| layer | OCI 镜像层，按 digest 内容寻址 |
| blob | OCI layout 中的二进制对象，layer/config/manifest 都是 blob |
| s-mart 包 | 当前上传格式：外层 `tar.gz`，内含 `.Version`、`app_desc.yaml`、`logo.png`、每个模块一个 `<module>.tgz` |
| 模块包 `<module>.tgz` | 单个模块的 OCI layout 归档 |
| 包体积 | 上传的 `s-mart.tar.gz` 压缩后字节数 |
| 镜像体积 | 解压后的 OCI 镜像层总量，不等于包体积 |
| 硬链接 | 多个文件名指向同一 inode；tar 可只存一份内容，解包后共享磁盘块 |

## 1. 背景与现状

SOPS 测试应用：

```
modules:
  default   sourceDir=./src  Python  3 processes
  callback  sourceDir=./src  Python  1 process
  pipeline  sourceDir=./src  Python  13 processes
  api       sourceDir=./src  Python  1 process
```

四个模块源码相同，但被构建四次，产出四个镜像。

当前实测：

| 指标 | 数值 |
|---|---|
| 模块数 | 4 |
| 单模块镜像层数 | 8 |
| 层引用总数 | 32 |
| 唯一层数 | 17 |
| 单模块镜像体积 | 约 129.5 MB |
| 唯一层总量 | 约 412.6 MB |
| 当前 s-mart 包体积 | 542 MB |

官方包同样存在重复：

| 官方模块包 | 大小 |
|---|---|
| default.tgz | 约 381 MB |
| api.tgz | 约 431 MB |

结论：这是**按模块独立打包**导致的系统性重复，不是本项目特有的构建问题。

## 2. 目标

### 2.1 主目标

1. SOPS 四模块 s-mart 包 **≤ 200 MB**
2. 四个模块仍可独立部署
3. 不修改应用源码
4. `app_desc.yaml` 保持完整模块声明
5. 构建时间不超过单模块的 1.5 倍

### 2.2 非目标

- 不合并业务模块
- 不牺牲模块独立发布/扩缩容
- 不改变运行时 launcher/Procfile 语义
- 不引入新的镜像仓库强依赖（可作为可选项）

## 3. 维护边界与约束

### 3.1 两侧职责

| 侧 | 归属 | 内容 |
|---|---|---|
| 构建侧 | **本项目维护** | `bksmartpacker`、buildpacks、pipeline、s-mart 生成 |
| 部署侧 | **BK PaaS，不由本项目维护** | 页面上传、包解包、镜像加载、模块启动 |

本文中的“平台”一律指**部署侧（BK PaaS）**，不是我们的构建方案。
本项目**不能修改部署侧**，只能验证其已有能力。

### 3.2 约束

| 约束 | 说明 |
|---|---|
| 部署平台校验 | 上传时 `app_desc` 声明的模块必须都有 `<module>.tgz`；单模块包会卡住 |
| 部署平台不可改 | 不能要求 BK PaaS 增加共享 blob、registry 部署等新能力 |
| 只可验证 | 只能通过测试包确认部署平台是否已支持硬链接、多 tag、共享层 |
| 包格式 | `.Version=cnb-image-layers`，外层 tar.gz，内层模块 tgz |
| 运行时 | 固定，不能假定会调用 `/cnb/process/<type>` |
| 上传方式 | 页面直传，速度慢；200 MB 是现实目标 |
| 应用源码 | 不可修改 |
| 架构 | amd64/arm64 都要满足 |

## 4. 业界参考

### 4.1 Heroku：一个 slug，多进程

Heroku 的模型是：

```
一个应用 → 一个 slug → Procfile 声明多个 process type
```

部署时按 process type 启动，不按进程拆包。参考：
[Heroku Procfile](https://devcenter.heroku.com/articles/procfile)

### 4.2 Paketo / CNB：一个应用一个镜像，多 process

CNB 用 launch.toml 声明多个 process：

- 一个镜像
- 多个 process type
- 进程不是镜像

参考：
[Specify launch processes](https://buildpacks.io/docs/for-buildpack-authors/how-to/write-buildpacks/specify-launch-processes/)

### 4.3 OCI / 容器仓库：按 digest 去重

OCI 镜像的 layer 是内容寻址的：

- 不同 tag 可以共享同一 layer
- registry 只存一份 blob
- manifest 引用 digest

参考：
[OCI image manifest](https://github.com/opencontainers/image-spec/blob/main/manifest.md)

### 4.4 Cloud Foundry / kpack

同样是一个应用一个 droplet/image，多进程共用。

### 4.5 共同点

> 镜像按“代码 + 依赖”划分，进程是镜像内元数据。

BK 的“模块”比 Paketo 的 app 更细，因此同一份源码被拆成四个镜像。

## 5. 候选设计

以下设计用 D1 ~ D5 编号，避免“方案 1/2”歧义。

### D1 单镜像多 manifest + 硬链接

**做法**

1. 四个模块只构建一次
2. 生成一个 OCI layout，包含四个 manifest，分别对应 `default/callback/pipeline/api` 的 tag，但共享全部 layer blob
3. 只生成一个 tgz
4. 外层 tar 中四个 `<module>.tgz` 作为硬链接指向同一份内容
5. 每个 tgz 内含全部四个 tag；平台按模块 tag 取镜像

**包体积估算**

```
一个镜像约 135 MB + 少量 manifest/config
硬链接后外层只存一份
目标 ≤ 200 MB：满足
```

**优点**

- 构建次数 4 → 1
- 包体积接近单镜像
- 不需要平台支持新格式（前提是解包保留硬链接）
- 不需要修改应用源码

**风险**

- 平台解包工具可能不保留硬链接
- 平台可能不接受“一个 tgz 内含多个 tag”
- 模块 tag 冲突/覆盖行为未知
- 需要先做小包验证

### D1 原理与限制（补充）

#### 原理

1. OCI 镜像由 `manifest + config + layers` 组成，layer 按 digest 内容寻址
2. 多个 tag 可以指向同一个 manifest digest
3. 一个 tgz 的 `manifest.json` 可以声明多个 `RepoTags`
4. 外层 tar 支持硬链接：多个 `<module>.tgz` 指向同一 inode，只存一份内容
5. 平台解包后四个模块共享一个 tgz，加载后四个 tag 指向同一镜像
6. 模块进程命令来自 `app_desc.yaml`，因此镜像不需要按模块拆分

#### 成立前提

| 前提 | 说明 |
|---|---|
| 源码相同 | 同 `sourceDir`，同代码 |
| 依赖相同 | 同语言、同 buildpacks、同构建期依赖 |
| 进程在 app_desc | 模块差异只体现在启动命令 |
| 平台保留硬链接 | 已在本平台验证 |
| 平台接受多 tag tgz | 已在本平台验证 |
| 按模块 tag 取镜像 | 已在本平台验证 |

#### 限制

| 限制 | 影响 |
|---|---|
| 只适合同源码/同依赖 | 不同依赖必须拆开构建 |
| 分组不校验镜像内容 | 当前工具按元数据分组，可能误合并 |
| 模块共享 digest | 无法独立升级/回滚/打补丁 |
| 依赖硬链接 | 平台重新打包或复制会丢失去重 |
| 依赖多 tag 加载 | 其他运行时可能只取第一个 tag |
| 镜像内 Procfile 只有主模块 | 若运行时不用 app_desc 会缺进程 |
| 仍是构建四次 | 当前工具只做打包后处理 |
| 平台要求模块齐全 | 不能少模块 |
| 多架构 | amd64/arm64 分别打包 |

#### 适用/不适用

| 场景 | 结论 |
|---|---|
| 同源码、不同进程 | 适用 |
| 不同语言/依赖 | 不适用 |
| 模块需独立版本 | 不适用 |
| 平台不支持硬链接/多 tag | 不适用 |
| 需要按模块不同基础镜像 | 不适用 |

### D2 构建一次 + Procfile 薄层

**做法**

1. 相同源码/依赖只构建一次 base 镜像
2. 每个模块追加一个只替换 `/app/Procfile` 的薄层
3. 四个镜像共享 base layer

**包体积估算**

- 构建时间：1x
- 包体积：**不会自动下降**，仍是四份独立 tgz
- 除非叠加 D1/D3，否则不满足 200 MB

**结论**：只解决构建时间，不作为体积方案。

### D3 s-mart 共享 blob

**做法**

```
s-mart/
  blobs/sha256/...        # 唯一 layer 只存一份
  manifests/default.json
  manifests/callback.json
  manifests/pipeline.json
  manifests/api.json
  app_desc.yaml
  .Version
```

**包体积估算**

- 仅共享当前四镜像：约 412 MB（只省 24%），**不满足 200 MB**
- 若叠加 D2（base 只构建一次 + 薄层）：约 135 MB，满足

**风险**

- 需要平台解包器支持共享 blob/多 manifest
- 属于 s-mart 格式扩展，需要平台配合

### D4 外部镜像仓库

**做法**

- 只构建/推送一次镜像
- s-mart 只带元数据或 digest
- 平台按 digest 拉取

**优点**

- 不依赖页面上传
- 镜像层由 registry 去重

**风险**

- 平台必须支持 registry 部署
- 需要凭证/网络

### D5 合并模块

**做法**

- 四个模块合并为一个模块，多个 process

**包体积**

- 约 135 MB

**结论**

- 破坏模块独立部署，不推荐
- 只作为最后手段

### D6 多镜像单 tgz（D1 的改进版）

**做法**

1. 组内只构建一次 base 镜像
2. 为每个模块生成一个薄层，只替换 `/app/Procfile` 等模块差异
3. 四个模块镜像**共享 base layer，但 digest 不同**
4. 一个 OCI 归档中放入四个 image manifest，按 tag 区分
5. 外层 tar 中四个 `<module>.tgz` 仍然硬链接到同一个多镜像归档
6. 平台按 tag 加载四个镜像

**包体积**

```
base ~135 MB + 四个薄层（几 MB）
硬链接后外层仍 ~136 MB
```

**与 D1 的区别**

| 项 | D1 | D6 |
|---|---|---|
| 镜像 digest | 四个模块相同 | 四个模块不同 |
| 镜像内 Procfile | 主模块的 | 每个模块自己的 |
| 独立升级 | 不可以 | 可以（各自薄层） |
| 构建次数 | 1 | 1 |
| 平台要求 | 多 tag | 多镜像归档 |

**本地验证（2026-11-24）**

- 构造 4 个不同 config、共享同一 layer 的 image manifest
- 一个 tgz 内放 4 个 manifest，`podman load` 成功
- 四个 tag 对应四个不同 image digest
- 测试包：

```
dist/linux-amd64/bk-sops-d6-tiny-4modules.tar.gz   # 19 KB，结构验证
dist/linux-amd64/bk-sops-d6-4modules.tar.gz        # 135.9 MB，真实镜像验证
```

生成脚本：

```bash
python3 script/tools/d6_test_package.py <s-mart.tar.gz>   --out bk-sops-d6-4modules.tar.gz
```

### D6 薄层修正（2026-11-25）

此前 D6 测试包只改了 config，镜像内 `/app/Procfile` 和
`/layers/config/metadata.toml` 仍是 default 模块的，导致 pipeline 的
`cworker` 进程找不到，运行时报：

```
exec: "cworker": executable file not found in $PATH
```

正确 D6 需要每个模块一个薄层，覆盖：

```
/app/Procfile
/layers/config/metadata.toml
/cnb/process/<process-type> -> /cnb/lifecycle/launcher
```

生成脚本：

```bash
python3 script/tools/d6_thin_package.py <s-mart.tar.gz>   --out bk-sops-d6-thin-4modules.tar.gz
```

测试包：

```
dist/linux-amd64/bk-sops-d6-thin-4modules.tar.gz   # 135.9 MB
```

本地验证：

- 四个模块 tag 加载成功
- pipeline 镜像的 `metadata.toml` 包含 `cworker` 等全部进程
- `/cnb/process/cworker` symlink 指向 launcher
- `/app/Procfile` 是 pipeline 的进程列表

注意：仍是测试构造，base 镜像复用 default；生产 D6 需要在构建时生成这些薄层。

**风险**

- 部署平台是否接受一个 tgz 内多个 image manifest（待上传验证）
- 加载后是否四个 tag 都能独立部署
- 薄层生成需要改写 OCI 镜像，复杂度高于 D1

### D7 镜像瘦身（正交方案）

**做法**

- 去掉 pip/uv 缓存
- 去掉 `.pyc`、测试、文档、编译工具
- 使用更小的基础镜像 / 多阶段构建
- 压缩层（gzip -9 / zstd）

**效果**

- 单镜像 135 MB 可能降到 80~100 MB
- 但四个模块仍然四份，**单独使用不足以达到 200 MB**
- 可与 D1/D6/D3 叠加

### D8 外部 Registry

**做法**

- 构建一次，推送镜像
- s-mart 只带元数据或 digest
- 平台按 digest 拉取

**优点**

- 不走页面上传
- registry 天然按 layer 去重

**限制**

- 部署平台必须支持 registry 部署
- 需要网络与凭证
- 不是我们可单方面实施的方案

### D9 仅压缩格式优化

**做法**

- 外层 tar 用 zstd/更高 gzip
- 利用长窗口压缩重复内容

**效果**

- 对 542 MB 去重有限
- 平台可能只接受 `.tar.gz`
- 不能替代镜像/包级去重

### D10 合并模块

**做法**

- 四个模块合并为一个模块、多个 process

**效果**

- 包体积 ~135 MB
- 不依赖平台新能力

**限制**

- 破坏模块独立部署
- 改变 `app_desc` 语义
- 仅业务接受时可用

## 6. 设计对比

| 设计 | 包体积 | 构建次数 | 部署平台依赖 | 应用改动 | 本项目可自主实施 |
|---|---|---|---|---|---|
| D1 单镜像 + 硬链接 | ~135 MB | 1 | **需验证**：硬链接解包、多 tag | 无 | 是（验证通过后） |
| D2 薄层 | 542 MB | 1 | 无 | 无 | 是（但不解决体积） |
| D3 共享 blob | 412 MB / 135 MB（叠加 D2） | 1 | **需部署平台已支持**，否则不可行 | 无 | 否 |
| D4 registry | 不上传 | 1 | **需部署平台已支持** | 无 | 否 |
| D5 合并模块 | ~135 MB | 1 | 无 | 语义变化 | 是（需业务接受） |
| D6 多镜像单 tgz | ~136 MB | 1 | **本地通过，待平台上传** | 无 | 是（需实现） |
| D7 镜像瘦身 | 单镜像 80~100 MB | 1~4 | 无 | 无 | 是（正交） |
| D8 registry | 不上传 | 1 | **需平台已支持** | 无 | 否 |
| D9 压缩格式 | 略降 | 1~4 | 平台需接受 | 无 | 是（收益小） |

## 7. 推荐路线

### 阶段 1：验证 D1 的平台兼容性（已完成）

用最小包验证三件事：

1. 外层 tar 硬链接解包后是否只占一份
2. 平台是否接受一个 tgz 内含多个 tag
3. 按模块部署时能否取到正确 tag

验证通过则采用 D1。

### 阶段 2：D1 实现（已用打包后处理验证）

当前 `script/tools/dedup_smart.py` 已实现 D1 的打包后处理版本：
549 MB → 135.9 MB，平台上传/部署通过。

### 阶段 2.5：评估 D6

D6 保留每个模块独立镜像 digest 和 Procfile，比 D1 更接近生产语义。
需要先构造多镜像单 tgz 的测试包，验证平台加载四个 tag。

### 阶段 3：D1 正式实现

在 bksmartpacker 中：

1. 识别可复用的模块组（sourceDir、language、buildpacks、构建期 env 相同）
2. 组内只构建一次
3. 生成共享 blob 的多 manifest OCI layout
4. 外层打包时对相同 tgz 使用硬链接
5. `app_desc` 保持完整

### 阶段 3：部署平台不支持 D1 时

D3 + D2 需要部署平台支持共享 blob/多 manifest；本项目不能推动改造。

因此只有两种选择：

1. 部署平台已经支持 → 按 D3 + D2 实施
2. 部署平台不支持 → 放弃 200 MB 目标，或走 D5（需业务接受模块合并）

不应把“升级平台解包器”作为本项目交付项。

## 8. 需求清单

### 功能需求

| 编号 | 需求 |
|---|---|
| F1 | 相同源码/依赖的模块组只构建一次 |
| F2 | 每个模块仍有独立 tag/产物 |
| F3 | `app_desc` 声明与产物一致 |
| F4 | 包内重复 layer 只存一份，或 tgz 硬链接 |
| F5 | 支持回退到全量独立构建 |
| F6 | 支持 `--module` 调试构建，但明确不可部署 |

### 非功能需求

| 编号 | 需求 |
|---|---|
| N1 | SOPS 四模块包 ≤ 200 MB |
| N2 | 构建时间 ≤ 单模块 1.5 倍 |
| N3 | 解包后各模块镜像可独立启动 |
| N4 | amd64/arm64 一致 |
| N5 | 产物可复现（相同输入相同 digest） |
| N6 | 不修改应用源码 |
| N7 | 不要求运维手工改包 |

## 9. 验收标准

1. `s-mart.tar.gz` 实际字节数 ≤ 200 MB
2. `tar -tvf` 能看到模块 tgz 为硬链接（或共享 blob 布局）
3. 上传不再卡住
4. 四个模块分别部署成功
5. `web/beat/dworker` 等进程正常启动
6. Go/Node 样例应用重复验证通过

## 10. 部署平台能力验证清单

| 编号 | 验证项 | 通过标准 |
|---|---|---|
| V1 | 外层 tar 硬链接 | 解包后多个模块 tgz 共享同一 inode，磁盘只占一份 |
| V2 | 单 tgz 多 tag | 上传/加载后四个模块 tag 都存在且可分别启动 |
| V3 | 按模块取镜像 | 部署 `api` 时使用 `bk_sops/api`，不回退到其他模块 |
| V4 | 模块缺失校验 | 确认当前必须全模块齐全（已通过单模块包验证） |

验证方式：构造最小两模块测试包，不涉及业务镜像。

## 11. 本地验证记录（2026-11-24）

### 11.1 测试包

| 用途 | 路径 | 大小 |
|---|---|---|
| 平台上传能力验证（快） | `dist/linux-amd64/bk-sops-d1-tiny-4modules.tar.gz` | 19 KB |
| 真实镜像结构验证 | `dist/linux-amd64/bk-sops-d1-test-4modules.tar.gz` | 136 MB |

**tiny 包**使用 `fake-cnb-runner` 的极小镜像，四个 tag 指向同一镜像，
只用于验证上传器是否接受硬链接、多 tag、四模块结构；启动进程会失败，
这是预期的。

结构：

```
.Version
app_desc.yaml       # 完整 4 模块
logo.png
api.tgz             # 实体文件
callback.tgz        # 硬链接 -> api.tgz
pipeline.tgz        # 硬链接 -> api.tgz
default.tgz         # 硬链接 -> api.tgz
```

镜像内容说明：四个 tag 指向**同一个 default 镜像**，仅用于验证包结构、
硬链接和 tag 加载；不代表生产镜像内容。

### 11.2 结果

| 编号 | 验证项 | 结果 | 说明 |
|---|---|---|---|
| V1 | 外层 tar 硬链接 | **本地通过** | GNU tar 解包后 4 个 tgz 共享同一 inode，磁盘只占一份；tiny 包与 136 MB 包均验证 |
| V2 | 单 tgz 多 tag | **本地通过** | `podman load` 一次加载出 default/callback/pipeline/api 四个 tag，image ID 相同；tiny 包与 136 MB 包均验证 |
| V3 | 按模块取镜像 | **待部署验证** | 四个 tag 均存在；上传已通过，BK 实际部署取哪个 tag 未验证 |
| V4 | 模块缺失校验 | 已知 | 单模块包上传页面卡住，平台要求模块齐全 |

### 11.3 复现方法

1. 取任一模块的 OCI layout（`<module>.tgz` 解包）
2. 在 `manifest.json` 的 `RepoTags` 写入四个模块 tag
3. 在 `index.json` 复制四条 manifest 引用，指向同一 manifest digest
4. 在 `repositories` 写入四个 tag 到同一 digest
5. 重新打成一个 `multi.tgz`
6. 外层 tar 中四个 `<module>.tgz` 作为硬链接指向 `multi.tgz`
7. 写入完整 `app_desc.yaml`、`.Version`、`logo.png`

### 11.4 平台上传结果（2026-11-24）

| 包 | 大小 | 上传结果 |
|---|---|---|
| `sops-d1-tiny-4modules.tar.gz` | 0.02 MB | **成功** |
| `bk-sops-d1-test-4modules.tar.gz` | 135.83 MB | **成功** |

结论：

- 部署平台接受外层 tar 硬链接
- 接受一个 tgz 内含四个模块 tag
- 不再出现“单模块包卡住”
- 包体积 135.83 MB，满足 ≤200 MB 目标

### 11.5 自动去重工具（D1 原型）

脚本：

```
script/tools/dedup_smart.py
```

用法：

```bash
python3 script/tools/dedup_smart.py dist/linux-amd64/s-mart.tar.gz \
  --out dist/linux-amd64/s-mart-d1.tar.gz
```

或：

```bash
just dedup
```

逻辑：

1. 读 `app_desc.yaml`
2. 按 `(sourceDir, language, buildpacks)` 分组
3. 组内取第一个模块的 OCI layout
4. 给同一个镜像追加组内全部模块 tag
5. 其余模块 tgz 改为硬链接
6. 外层 tar 保留硬链接

实测：

| 指标 | 数值 |
|---|---|
| 输入 | 549 MB（4 模块独立镜像） |
| 输出 | **135.9 MB** |
| tgz 数量 | 4，inode 相同 |
| 加载 tag | default/callback/pipeline/api |
| `podman load` | 四个 tag 均可加载 |

产物：

```
dist/linux-amd64/bk-sops-d1-auto-4modules.tar.gz
```

### 11.6 仍未验证

- BK 上传器是否保留外层 tar 硬链接（可能复制文件导致体积反弹）
- BK 是否接受“一个 tgz 内含多个 tag”
- BK 部署 `api` 时是否使用 `bk_sops/api` tag
- 硬链接包在 arm64 上的一致性

以上只能通过部署平台上传测试确认，本项目无法在本地替代。

建议先上传 **tiny 包**（19 KB）验证结构；通过后再上传 136 MB 包验证真实镜像。

## 12. D1 union 最终验证（2026-11-25）

### 12.1 平台实际镜像

用 skopeo 拉取四个模块 tag 的 manifest：

```bash
skopeo inspect --tls-verify=false --raw   docker://docker.dev-x86.cwbk5.com/bkpaas/docker/bk_sops/pipeline:3.34.1-beta.20
```

四个 tag 的顶层 layer 都是：

```
sha256:d3eae8791e757d3b5946c03cecf748179564a22a595b2f24f769288d365af454
```

该 digest 与本地 D1 union 包的顶层 layer **完全一致**。

说明平台确实把四个模块都指向了同一个 union 镜像。

### 12.2 union 镜像内容

顶层 layer 内：

```
/layers/config/metadata.toml
  包含 node-timeout、cworker、v1-engine、api-er-*、web（pipeline 命令）

/app/Procfile
  包含全部 process type

/cnb/process/<type>
  指向 /cnb/lifecycle/launcher
```

### 12.3 部署结果

| 模块 | 结果 |
|---|---|
| default | 部署成功、Pod 正常 |
| callback | 部署成功、Pod 正常 |
| pipeline | 部署成功、Pod 正常 |
| api | 部署成功、Pod 正常 |

验证方式：用户实际部署 + skopeo 拉取镜像对比。

### 12.4 结论

- 平台只加载一个镜像
- 不需要多 image manifest
- 需要把所有 process type 放进同一个镜像 metadata
- D1 union 满足 ≤200 MB 目标（135.9 MB）
- D6 多镜像单 tgz 不可行

测试包：

```
dist/linux-amd64/bk-sops-d1-union-4modules.tar.gz
```

生成脚本：

```bash
python3 script/tools/d1_union_package.py <s-mart.tar.gz>   --out bk-sops-d1-union-4modules.tar.gz
```

注意：同名进程（如 `web`）只能保留一个命令；当前 union 以 pipeline 为准。
生产实现应在构建阶段生成 union metadata，而不是打包后补层。

## 13. 风险与待定

| 风险 | 处理 |
|---|---|
| 部署平台不保留硬链接 | D1 不可行；D3 需平台已支持，否则目标不可达 |
| 部署平台不接受多 tag tgz | 同上 |
| 构建期 env 不同导致镜像不可复用 | 仅对构建输入完全相同的模块分组 |
| 模块 tag 冲突 | 先做小规模验证 |
| 200 MB 目标无法在不改部署平台下达成 | 记录为平台能力限制，不作为本项目承诺 |

## 14. 官方 v2 方案

官方 smart-app-builder v2 用 artifact.json 解决同一问题，详见：

[smart-app-v2-analysis.md](../design/projects/offline-paketo-bk/smart-app-v2-analysis.md)

核心结论：

- v2 没有改变平台“一个 tar 只取第一个 manifest”的行为
- v2 改为一个构建组一个 image tar
- artifact.json 负责 image_tar 共享和进程 entrypoint 映射

## 15. 决策记录

- 目标：SOPS 四模块包 ≤ 200 MB
- 首选：D1（单镜像多 manifest + 硬链接）
- 备选：D3 + D2（仅当部署平台已验证支持）
- 不做：D5 合并模块（除非明确接受语义变化）
- 当前状态：D1 union 端到端验证通过；D6 否决
- 最终方案：单镜像 + union metadata + 四模块 tag
- 上传结果：tiny 0.02 MB、full 135.83 MB，均成功
- 自动去重工具：549 MB → 135.9 MB
- 后续：并入 bksmartpacker，减少重复构建
- 测试包：`dist/linux-amd64/bk-sops-d1-test-4modules.tar.gz`，136 MB
- 边界：不修改、不推动修改 BK PaaS
