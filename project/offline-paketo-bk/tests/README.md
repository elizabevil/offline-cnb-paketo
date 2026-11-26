# offline-paketo-bk 测试

## 入口

这些脚本由项目 `justfile` 调用，不直接面向使用者。

```bash
just test-quick
just test-export-modes
just test-runtime-matrix
```

覆盖：

```bash
just test-quick app=/path/to/app platform=linux/arm64
```

## 文件

| 文件 | 用途 |
|---|---|
| `quick.sh` | 开发期快速打包验证 |
| `export-modes.sh` | layout / daemon 对比 |
| `matrix.sh` | runtime 组合入口 |

## 历史

原 `project/offline-paketo-bk/run-test.sh` 是开发期快速验证脚本，
只用于 CMDB SaaS 打包，不作为 CI 矩阵。

迁移后对应 `quick.sh`，旧路径删除。
