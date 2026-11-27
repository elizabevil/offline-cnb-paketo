# offline-paketo-bk 使用说明

> 本目录只放**使用**。需求、架构、决策在
> [`docs/design/offline-paketo-bk/`](../../../docs/design/projects/offline-paketo-bk/README.md)。

## 常用命令

在仓库根执行：

```bash
just --justfile project/offline-paketo-bk/justfile <recipe>
```

推荐的非 root 测试入口：

```bash
just test-quick          # host-podman + non-root + layout
just test-pind           # pind + non-root + layout
just test-dind           # dind + non-root + layout
just test-dind-daemon    # dind + non-root + daemon
```

根目录转发：

```bash
just test-bk-quick
just test-bk-pind
just test-bk-dind
just test-bk-dind-daemon
```

打包应用：

```bash
just smart /path/to/app
```

构建 builder：

```bash
just smart-builder
```

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `BKS_APP` / `S_MART_SRC` | `fixtures/single` | 应用目录 |
| `BKS_PLATFORM` | 宿主架构 | `linux/amd64` 或 `linux/arm64` |
| `BKS_BUILDER_USER` | `1000:1000` | builder 用户 |
| `BKS_BUILDER_USERNS` | `keep-id` | builder userns |
| `CNB_CACHE_DIR` | `.cache/bk-cache` | 跨构建缓存 |

## 产物

```text
dist/<platform>/s-mart.tar.gz
.cache/tests/offline-paketo-bk/<platform>/
```
