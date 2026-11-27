# offline-paketo-bk 测试

## 推荐入口

默认使用非 root builder：

```bash
just test-quick                 # host-podman + non-root + layout
just test-pind                  # pind + non-root + layout
just test-dind                  # dind + non-root + layout
just test-dind-daemon           # dind + non-root + daemon
```

指定应用和平台：

```bash
just test-quick app=/path/to/app platform=linux-arm64
```

根目录转发：

```bash
just test-bk-quick
just test-bk-pind
just test-bk-dind
just test-bk-dind-daemon
```

## 默认值

```text
app       fixtures/single
platform  linux-amd64
builder   1000:1000
userns    keep-id
```

`just smart` 也默认注入：

```text
BKS_BUILDER_USER=1000:1000
BKS_BUILDER_USERNS=keep-id
```

需要覆盖时：

```bash
BKS_BUILDER_USER=0 BKS_BUILDER_USERNS= just smart
```

## 脚本

| 文件 | 用途 |
|---|---|
| `quick.sh` | host-podman 快速验证 |
| `managed.sh` | pind/dind 非 root 验证 |
| `export-modes.sh` | layout/daemon 对比 |
| `matrix.sh` | 矩阵入口 |

`managed.sh` 使用 `/tmp/cnb-managed-*` 短路径作为 workspace；Unix socket
路径超过 108 字符会连接失败。

## 产物

```text
.cache/tests/offline-paketo-bk/<platform>/
```

命名：

```text
s-mart-<runtime>-<export>-<arch>.tar.gz
```

## 历史

原 `run-test.sh` 已迁移为 `quick.sh`。
