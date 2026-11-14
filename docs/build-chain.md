# 构建链路与职责划分

> 适用：`project/offline-paketo-bk`（bksmartpacker 契约 + 自研 paketo 组件）。
> 目的：把"谁负责什么、产物落在磁盘哪个位置"固定下来，后续改动按此定位。

## 一、角色与职责

| 角色 | 职责 | 不负责 | 是否访问 daemon |
|---|---|---|---|
| bksmartpacker（本地补丁） | 打包源码、拼 `docker run` 参数、导出 fake runner、收集层包、组装 s-mart | 不解析 buildpack、不调 lifecycle | 是（`image save`） |
| entrypoint（容器内 3 行） | 交出控制权给 `cnb` | 无编排逻辑 | 否 |
| cnb lifecycle run | 解包源码、过滤 order、按序执行阶段、生成 analyzed.toml、转换 layout→docker-archive、降权 | 不实现 buildpack 逻辑、不组装 s-mart | 否（run image 由归档提供时） |
| detector | 按 order 选择 buildpack | 不产出镜像 | 否 |
| builder | 执行 buildpack 的 build 脚本，产出 layers | 不产出镜像 | 否 |
| analyzer | 读 run image 元数据写 analyzed.toml | 不产出镜像 | 是（daemon 模式，当前默认跳过） |
| exporter | 把 layers + run image 写成应用镜像 | 不读 daemon（layout 模式） | 否 |

## 二、磁盘产物

| 阶段 | 产物 | 位置 |
|---|---|---|
| 源码解包 | 应用文件 | `/app` |
| order 过滤 | `order.toml` | `/layers/order.toml` |
| detect/build | layers、`launch.toml`、SBOM | `/layers/<buildpack-id>/` |
| run image 导入 | OCI layout | `/layers/oci/index.docker.io/library/<repo>/<tag>/` |
| 元数据 | `analyzed.toml`（reference + top-layer） | `/layers/analyzed.toml` |
| 导出 | 应用镜像 layout | `/layers/oci/index.docker.io/<repo>/<tag>/` |
| 层包 | docker-archive | `/cnb-layer/<module>.tgz` → 宿主 `<workspace>/layers/` |
| 最终产物 | s-mart 包 | bksmartpacker `--dest` |

## 三、数据流

```
源码目录
  └─(bksmartpacker 打包)─▶ source.tgz ──挂载─▶ /tmp/source.tgz
                                                  │
                                   cnb lifecycle run：解包 → /app
                                                  │
              REQUIRED_BUILDPACKS ──过滤──▶ /layers/order.toml
                                                  │
                              detector → builder → （analyzer 可跳过）
                                                  │
  run-image.tar ──挂载──▶ 转 OCI layout ──▶ analyzed.toml
                                                  │
                                    exporter -layout ──▶ layout 目录
                                                  │
                             cnb：layout → docker-archive
                                                  │
                         /cnb-layer/<module>.tgz ──挂载─▶ 宿主 workspace/layers/
                                                  │
                                   bksmartpacker 组装 s-mart.tgz
```

## 四、关键约束

| 约束 | 说明 |
|---|---|
| Platform API ≥ 0.12 | 低于该版本 lifecycle 不注册 `-layout`，layout 模式会自动抬到 0.12 |
| `CNB_EXPERIMENTAL_MODE=warn` | OCI layout 在 lifecycle 0.21 仍属实验特性 |
| analyzer 与 layout | analyzer 走 daemon 时只写得出镜像 ID；layout 模式下改为由磁盘 layout 直接生成 analyzed.toml，跳过 analyzer |
| run image 必须有层 | `FROM scratch` + `COPY` 可能得到 0 层镜像，bksmartpacker 改用 `image import` 构造单层镜像 |
| SDK 不入镜像 | Go/Node/Python 运行时通过 `CNB_RUNTIMES` 挂载或由组件下载到 `/layers/cache` |

## 五、失败归属

| 现象 | 归属 |
|---|---|
| `docker run` 参数错误、挂载缺失 | bksmartpacker |
| `未找到 buildpack: X` | cnb（order 过滤 / 别名映射） |
| `-layout not defined` | cnb（Platform API 下限） |
| `experimental features disabled` | cnb（实验开关） |
| `image has no layers` | bksmartpacker（fake runner 构造） |
| `run image not found in analyzed metadata` | cnb（analyzed.toml 生成时机） |
| buildpack 内部报错 | 对应组件（`buildpacks/paketo/*`） |
| s-mart 结构不对 | bksmartpacker |

## 六、analyzed.toml 的真实字段（从 lifecycle 0.21.29 二进制提取）

```
strings lifecycle | grep 'toml:"'
  RunImage   toml:"run-image"
  Reference  toml:"reference"
  TopLayer   toml:"top-layer"
```

因此 cnb 直接生成的格式为：

```toml
[run-image]
  reference = "fake-cnb-runner:latest@sha256:<manifest-digest>"
  top-layer = "sha256:<最后一层 digest>"
```

相关报错对照：

| 报错 | 含义 |
|---|---|
| `No run metadata found at path "/cnb/run.toml"` | 没有 run.toml，需要 `-run-image` 或 analyzed 元数据 |
| `missing run image metadata (-run-image)` | 同上 |
| `get run image top layer SHA: image has no layers` | analyzed 里 top-layer 为空，或 layout 中该镜像零层 |

## 七、镜像导出（BK 链路）

BK 场景下 cnb 只用显式配置决定导出方式，不感知平台：

```
CNB_EXPORT_MODE=daemon（由 bksmartpacker 注入）
        ↓
detect → build → analyze → export -daemon   # 镜像写进容器运行时
        ↓
bksmartpacker: image save -o <module>.tgz   # 产出 OCI 归档
        ↓
s-mart: .Version + app_desc + <module>.tgz + artifact.json
```

- 导出由 lifecycle exporter 完成，config / Entrypoint / buildpack 层都由它生成
- cnb 不读 `BKS_*`；平台差异全部留在 bksmartpacker
- 需要不依赖 daemon 时用 `--export=layout --layout-dir=...`
