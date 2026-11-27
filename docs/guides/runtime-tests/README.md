# Runtime 测试参考

> runtime 组合的测试矩阵与覆盖情况。属于**测试参考**，不是设计。
> 概念与架构见 [design/runtime/](../../design/runtime/README.md)。

| 文档 | 内容 |
|---|---|
| [matrix.md](matrix.md) | 总览、8 组合、未完成项、singleton |
| [rootful.md](rootful.md) | Outer root = root 的测试结果 |
| [nonroot.md](nonroot.md) | Outer root = non-root 的测试结果 |

测试产物与校验和不单独维护文档，以生成物为准：

```text
.cache/tests/full/<platform>/SHA256SUMS
```
