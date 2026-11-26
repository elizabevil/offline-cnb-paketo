# lifecycle singleton 连接复现器

复现 `buildpacks/lifecycle` 的 singleton Docker 客户端问题：

- 客户端只保留一条预先打开的 Unix socket 连接
- 当第一个请求（`ImageLoad` 流）尚未结束、又发起第二个请求（`ImageSave`）时
- 直接返回 `singleton connection already in use`

## 用途

验证 lifecycle 新版本是否修复该问题：如果修复，`ImageSave error` 不再是 singleton 报错。

## 运行

```bash
cd script/tools/cnb/tests/lifecycle-singleton
go mod tidy          # 首次需要网络
sudo go run .
```

必须以 root 运行：`priv.shouldConnectSock()` 只在 socket 属于 uid 0 时启用 singleton 路径。
非 root 运行会打印 `ImageSave error: <nil>`，属于预期。

## 期望输出（未修复时）

```
ImageSave error: error during connect: Get "http://%2Ftmp%2Flc-singleton-repro.sock/v1.41/images/get?names=sha256%3A000...":
singleton connection already in use
```

## 相关

- https://github.com/buildpacks/lifecycle/issues/1650
- https://github.com/buildpacks/lifecycle/pull/1757
- lifecycle `priv/docker.go`
