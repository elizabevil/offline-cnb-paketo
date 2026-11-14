# cnb — 基座小工具

单模块、单二进制、子命令式。**纯数据处理进这里，调度外部进程留 shell。**

```
script/tools/cnb/
├── main.go       # CLI（cobra）：子命令、flag、help、退出码
├── lock.go       # lock 文件模型、list/get/validate/platforms
├── bundle.go     # 物料路径、marker、verify、pin
├── repack.go     # web.tgz → OCI layout（tar/gzip/JSON）
├── cnb_test.go    # 命令契约单测
├── repack_test.go # repack golden 测试（内建夹具，无外部依赖）
└── go.mod/go.sum  # 唯一依赖 BurntSushi/toml v1.6.0（不使用 vendor）
```

产物：`script/bin/cnb-<os>-<arch>`（如 `cnb-linux-amd64`，静态链接、CGO 关闭）。由 `just tools` 生成，不入库。

---

## 一、命令契约（改动前必读）

| 命令 | 输出 | 退出码 | 谁依赖 |
|---|---|---|---|
| `list` | TSV：`id\tversion\tkind\tplatforms`，**列序固定** | 0 | 终端/脚本（`cut -f1`） |
| `get <id> <platform>` | shell 赋值 8 行，字段名固定 | 0 / 2 | `just` 配方 |
| `validate` | 成功一行 `✓`；失败逐条 `✗` 到 stderr | 0 / 1 | `just check`、CI |
| `platforms` | 逗号分隔 | 0 | 人工 |
| `verify [ids...]` | 逐物料 `✓/✗` | 0 / 1 | `just verify`、CI 门禁 |
| `pin [ids...]` | TOML 片段（不写回） | 0 / 1 | 人工回填 |
| `repack <dir\|*.tar.gz>` | 无（原地改写） | 0 / 1 | `offline-heroku-bk/apps/build.sh` |

约定：

- `get` 的字段名与顺序是契约，shell 侧用 `eval` 消费；改名要同步改脚本。
- `verify` 读 `bundle/<platform>/binaries/.meta/<id>.sha256`（marker），
  marker 由 `materials sync` 在解压后写入，内容为归档 sha256 或 `unpinned`。
- 全局选项（`--file/--bundle/--platform/--strict`）可出现在子命令前后任意位置。

---

## 二、新增一个子命令（模板）

以新增 `size`（统计某平台物料体积）为例，**四步**：

**1. 在 `main.go` 的 `run()` 里注册**

```go
switch cmd {
// ...
case "size":
    if err := printSizes(lock, bundle, platform, os.Stdout); err != nil {
        fmt.Fprintf(os.Stderr, "cnb: %v\n", err)
        return 1
    }
}
```

**2. 在合适的文件里实现**（解析类放 `lock.go`，物料类放 `bundle.go`，别新建文件）

```go
// printSizes 打印每个物料在当前平台的归档体积。
func printSizes(lock *lockFile, bundle, platform string, w io.Writer) error {
	assets, err := selectAssets(lock, nil) // nil = 全部
	if err != nil {
		return err
	}
	for i := range assets {
		a := &assets[i]
		size, err := fileSizeOf(archivePath(bundle, platform, a))
		if err != nil {
			fmt.Fprintf(w, "%-12s %s\n", a.ID, "缺失")
			continue
		}
		fmt.Fprintf(w, "%-12s %d\n", a.ID, size)
	}
	return nil
}
```

**3. 在 `usage()` 里加一行说明**（否则下一个人不知道有这命令）

```
  size                       各物料归档体积
```

**4. 补一个单测**（`cnb_test.go`），并跑：

```bash
gofmt -l . ; go vet ./... ; go test ./...
```

规则：

- 不改已有命令的输出格式；确需改，**同时**更新依赖它的脚本与 `docs/design/base.md`。
- 需要网络/容器/外部命令的，不要放进来（这类留在 shell）。
- 新增第三方依赖前先问：stdlib 能不能做？现在全仓只有 1 个依赖。

---

## 三、新建一个项目（模板）

```bash
script/bin/cnb-linux-amd64 profile init my-proj --dir my-proj       # 生成 config/projects/my-proj.toml
script/bin/cnb-linux-amd64 profile show config/projects/my-proj.toml # 分组查看（不平铺）
script/bin/cnb-linux-amd64 profile validate config/projects/my-proj.toml
```

模板只写结构与注释，物料/镜像/命令留成占位，避免猜业务。
`profile show` 按「项目 / 构建 / 物料 / 镜像 / 命令」分组输出。

## 四、新增一个物料（不动代码）

分两步，都在配置里：

1. `assets.lock.toml` 增加物料本体（版本 / URL / sha256 / 平台），
   模板见仓库根 [`assets.lock.toml.example`](../../../assets.lock.toml.example)。
2. 项目 profile 在 `[assets.<id>]` 下声明引用（键名即 id）：

```toml
[assets.lifecycle]
version   = "0.21.29"        # 省略则取 lock 的 default 版本
platforms = ["linux-amd64"]  # 省略表示全部平台
```

`cnb profile validate` 会与 lock 交叉校验：id / 版本 / 平台必须真实存在，
写错会直接列出可选值。

流程规则（`[[stage]]`）：

- 顺序即数组顺序；每步只能引用一个已定义的 `image` 或 `command`
- 定义了却没进流程的镜像/命令会报错，防止漏写
- `${image.<n>.tag}` 只能引用**流程中更早出现**的镜像，反序会直接报错

流程：填 lock → `just fetch <id>@<version>` → `just pin <id>` → 回填 sha256 → `just verify`。

---

## 五、构建与自测

```bash
just tools     # 编译到 script/bin/cnb-<os>-<arch>
just check     # lock 结构 + 模板合法性 + bash -n + gofmt + vet + 单测
go test ./...  # 只跑单测
```

`GOCACHE` 默认被指向仓库内 `.cache/go-build`：CI/沙箱里 `HOME` 只读时，
Go 默认缓存路径会直接 `read-only file system` 失败（已实测踩过）。

---

## 五、repack 的特别注意

- **JSON 键顺序即契约**：结构体字段顺序 = 输出键顺序，不要为了"整洁"重排。
- **目录必须递归写出**：`addTree` 对目录要展开子项，否则生成的包里只有空目录。
- **幂等**：输入已是 OCI layout（有 `oci-layout` + `index.json`）时直接返回，不重写文件。
- 改完必须跑 `go test -run Repack`；golden 常量在 `repack_test.go` 顶部，若确属预期变更，
  需连同**新的字节**一起 review，而不是简单覆盖。

## 命令分组（v3 起）

```
cnb materials  list|get|platforms|validate|verify|pin|publish|lock set-sha
cnb pipeline   plan|run|doctor|dockerfile check|profile …|repack
cnb buildpack  lint|metadata
cnb lifecycle  run
cnb version
```

旧的顶级命令（`list`/`get`/`verify`/`run`/`plan` 等）已移除，脚本与文档同步更新。
