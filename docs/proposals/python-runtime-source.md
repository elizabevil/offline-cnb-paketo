# Python 运行时来源（待办）

> 状态：**提案**。先用于离线验证，最终方案待定。

## 目标

不修改应用源码，不要求使用者手工设变量，Python 运行时开箱可用；优先离线。

## 候选方案

| 方案 | 做法 | 优点 | 代价 |
|---|---|---|---|
| A. 脚本内置默认 + 回退链 | `gh-proxy.com` → `ghproxy.net` → GitHub 直连，超时 600s，环境变量可覆盖 | 改动小 | 依赖公网速度 |
| B. builder 镜像 ENV | Dockerfile 固化 `UV_PYTHON_INSTALL_MIRROR`、`UV_HTTP_TIMEOUT`、`PIP_INDEX_URL` | 所有语言共享 | 换源要重建 builder |
| C. 本地目录镜像 | bundle 放 `python/<version>/`，`UV_PYTHON_INSTALL_MIRROR=file://...` | 完全离线、快 | 需要纳入物料 |
| D. runtime 物料化 | python-build-standalone 作为 asset（amd64/arm64） | 可复现、有哈希 | 版本矩阵变大 |
| E. 自建制品库 | 内网 HTTP 提供 tarball | 稳定 | 需要运维设施 |

## 建议

- 短期：A + B
- 长期：C / D

## 内置默认（候选）

| 变量 | 默认值 |
|---|---|
| `BP_CPYTHON_VERSION` | `3.12`（应用有 `runtime.txt` 时以它为准） |
| `UV_PYTHON_INSTALL_MIRROR` | `https://gh-proxy.com/https://github.com/astral-sh/python-build-standalone/releases/download` |
| `UV_HTTP_TIMEOUT` | `600` |
| `PIP_INDEX_URL` | `https://pypi.tuna.tsinghua.edu.cn/simple` |

## 验证记录

- `gh-proxy.com` 无本地代理：21,205,300 bytes / 63s / 约 335 KB/s
- `ghproxy.net`：90s 内未完成
- uv-custom 镜像的文件名是 `-plus-<build>`，原生 uv 不接受，不作为内置默认

## 待办

- [ ] 选定短期方案并实现内置默认
- [ ] 回退链的失败日志
- [ ] 评估 Python runtime 纳入物料
