# 构建变量注入方案（平台 env 目录）

> 状态：**提案**。用于解决 buildpack 收不到业务环境变量的问题。

## 一、问题

`bksmartpacker` 当前把 `app_desc` 的 `configuration.env` 和 `--env` 都通过
`docker run -e KEY=VALUE` 传入构建容器，但 lifecycle 在调用 buildpack 前会按白名单
过滤环境变量：

```go
// buildpacks/lifecycle@v0.21.29 env/build.go
var BuildEnvIncludelist = []string{
    "CNB_STACK_ID", "HOSTNAME", "HOME",
    "HTTPS_PROXY", "https_proxy",
    "HTTP_PROXY", "http_proxy",
    "NO_PROXY", "no_proxy",
}
```

结果：

- `app_desc` 的 `PIP_VERSION`、`PIP_INDEX_URL` 等业务变量不会到达 buildpack
- `--env` 注入的 `UV_*`、`PIP_*` 同样被丢弃
- 只有控制类变量和代理变量生效

Go 之前没暴露，是因为它把配置写进 builder 文件（`/cnb/go-mirror`），不依赖环境变量。

## 二、目标

1. `app_desc.configuration.env` 与 `--env KEY=VALUE` 都能到达 buildpack
2. 不修改应用源码
3. 保持 `REQUIRED_BUILDPACKS`、`CNB_PLATFORM_API`、`CNB_EXPORT_MODE` 等控制变量的
   `-e` 传递方式不变
4. 支持未来按构建覆盖默认值

## 三、业界方案

| 方案 | 做法 | 使用方 |
|---|---|---|
| A. 平台 env 目录 | `/platform/env/<KEY>`，lifecycle 读取后注入 buildpack | `pack --env`、kpack `build.env`、Tekton |
| B. builder 内置默认 | 默认值写镜像文件/ENV，构建时覆盖 | Paketo builder |
| C. buildpack 自定义文件 | 如 `/cnb/go-mirror` | 自研 buildpack |
| D. `project.toml` | 应用侧声明 build env，平台转换 | Paketo |
| E. service binding | 挂载文件，适合密钥 | 凭证场景 |
| F. entrypoint 转换 | 从容器 ENV 写 platform env | 补丁，不推荐 |

## 四、选定方案

**A + B**：

- A：`bksmartpacker` 生成 platform env 目录并挂载 `/platform`
- B：通用默认值仍放 builder（镜像文件或 ENV），由 A 覆盖

C 保留给已有 buildpack（Go），不强制迁移。

## 五、设计

### 输入

- `app_desc.yaml` 的 `modules[].spec.configuration.env`
- CLI `--env KEY=VALUE`（可重复）
- 合并规则：`--env` 覆盖 `app_desc` 同名项

### 执行

1. 每个模块构建时创建临时目录：

   ```
   <tmp>/platform/env/<KEY>    # 文件内容为变量值
   ```

2. 挂载：

   ```
   --mount type=bind,source=<tmp>/platform,target=/platform,readonly
   ```

3. 控制变量继续 `-e`：

   ```
   REQUIRED_BUILDPACKS, OUTPUT_IMAGE, CNB_RUN_IMAGE, SOURCE_GET_URL,
   USE_DOCKER_DAEMON, CNB_PLATFORM_API, CNB_EXPORT_MODE,
   CNB_LAYER_ARCHIVE, CNB_RUN_IMAGE_ARCHIVE
   ```

4. 不再为业务变量生成 `-e`

5. 构建结束删除临时目录

### 变量约束

- key 必须匹配 `[A-Za-z_][A-Za-z0-9_]*`
- value 原样写入（不展开、不加引号）
- 空值写空文件
- 含换行的值暂不支持，报错

### 优先级

```
运行时 --env
  > 项目 build.env
  > app_desc configuration.env
  > bksmartpacker buildpack.yaml（如保留）
  > builder 内置默认（文件/ENV）
  > buildpack 代码默认
```

## 六、bksmartpacker 配置收敛

现状：`--buildpack-cfg`（`conf/buildpack.yaml`）同时承担两件事：

1. 选择 BK buildpack 及版本（`bk-buildpack-python v213` 等）
2. 生成业务环境变量（`PIP_INDEX_URL`、`PIP_EXTRA_INDEX_URL`、`GO_BUCKET_URL` 等）

第 2 项目前通过 `-e` 传入，同样被 lifecycle 丢弃；而且默认值指向腾讯源，会与项目配置冲突。

建议：

| 配置 | 处理 |
|---|---|
| buildpack 选择（name/version/type） | 保留，仅在使用官方 BK builder 时有意义 |
| 业务环境变量（PIP_*、GO_BUCKET_URL、NPM_REGISTRY…） | 从 buildpack.yaml 移除，统一由 `build.env` 提供 |
| 本地 Paketo builder 路径 | 忽略 BK buildpack 版本字段，别名只做 id 映射 |

目标：

- 业务变量只有一个入口：项目 `build.env`
- `buildpack.yaml` 只描述“用哪些 BK buildpack”，不描述镜像源
- 运行时覆盖只走 `--env`

迁移：

1. 删除 `conf/buildpack.yaml` 中的镜像/索引字段
2. 项目 `build.env` 补上需要的值
3. `BuildpacksEnvs()` 不再生成业务变量
4. 文档标注 `buildpack.yaml` 仅用于官方 BK builder

## 七、影响面

| 文件 | 改动 |
|---|---|
| `pkg/builder/executors/docker.go` | 生成 platform env 目录、挂载、移除业务变量 `-e` |
| `pkg/builder/step.go` | 区分控制变量与平台变量（可选） |
| `README` / `ARCHITECTURE` | 说明 env 注入路径 |
| 测试 | fake docker 校验 `--mount target=/platform`、无业务 `-e` |

## 八、验证

1. 单元/fake docker：参数中不含 `-e PIP_*`，含 `/platform` 挂载
2. 构建 SOPS：
   - pip 日志出现 `index`、`find-links`、`no-index`
   - `cw-cornerstone` 从本地 `pkgs/` 解析成功
3. Go 构建回归：`BP_GO_VERSION` 仍生效（文件兜底不破坏）
4. arm64 构建不受影响

## 九、待定

- 是否需要 `--env-file`（从文件批量读取）
- 是否把 platform env 目录持久化以便排错（默认删除）
- Go/Node 是否迁移到 platform env，还是保留文件兜底

## 十、不做

- 不修改应用源码
- 不修改 buildpack 现有变量读取逻辑
- 不改变 `pack`/CNB 标准约定
