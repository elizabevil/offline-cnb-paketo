# buildpack 拆分：进度与迁移记录（已完成）

> 状态：**历史**。拆分已完成，设计见 [`docs/design/buildpack-components.md`](../design/components/buildpack-components.md)。

## 一、进度

| 语言 | 组件 | 状态 |
|---|---|---|
| node | node-engine 8.12.0 / npm-install 2.3.36 / npm-start 2.5.13 / nodejs(meta) | ✅ e2e 通过 |
| python | cpython 1.18.51 / pip-install 0.7.51 / python(meta) | ✅ e2e 通过 |
| go | go-dist 2.11.2 / go-build 2.4.48 / go(meta) | ✅ e2e 通过 |
| procfile | 官方 5.13.4（预编译产物） | ✅ e2e 通过 |

e2e 覆盖：`just test-e2e-core-all`（4 个用例，均 HTTP 200）。

### 下载源（可配置，国内默认）

| 组件 | 变量 | 默认 |
|---|---|---|
| go-dist | `BP_GO_MIRROR` | `https://mirrors.aliyun.com/golang/` |
| cpython | `UV_PYTHON_INSTALL_MIRROR` | gh-proxy 加速的 python-build-standalone |
| pip-install | `UV_INDEX_URL` | `https://pypi.tuna.tsinghua.edu.cn/simple` |
| npm-install | `NPM_CONFIG_REGISTRY` | `https://registry.npmmirror.com` |

## 二、当时的迁移步骤

1. **node**：拆 `node-engine` / `npm-install` / `npm-start` + `nodejs` meta，
   e2e 通过后删除旧的单包 `node`
2. **python**：拆 `cpython` / `pip-install` + `python` meta
3. **go**：拆 `go-dist` / `go-build` + `go` meta
4. 更新 `order.toml` 只引用三个 meta
5. `cnb buildpack lint` 接入 `script/ci.sh`

## 三、不做的部分

- 不引入官方产物（不再从 builder 镜像复制 buildpacks）
- 不做动态版本解析（`[[metadata.dependencies]]` 只声明锁定版本）
- 不支持多架构脚本（沿用现有单架构 + 平台参数）
