# 子项目 justfile 契约

`project/<name>/justfile` 只做**入口转发**，流程与版本全部来自
`config/projects/<name>.toml`。三个项目遵循同一套 recipe。

## 一、统一变量

```make
_dir     := justfile_directory()
_root    := _dir / ".." / ".."
_cnb     := _root / "script" / "bin" / "cnb"
_profile := _root / "config" / "projects" / "<name>.toml"
```

禁止在 recipe 里写 `{{_dir}}/../../script/bin/cnb` 这类重复的相对路径。

## 二、必备 recipe

| recipe | 行为 |
|---|---|
| `build *args` | `cnb pipeline run <profile> <args>` |
| `plan *args` | `cnb pipeline plan <profile> <args>`（yaml，`--full` 展开） |
| `fetch *ids` | `cnb materials sync <ids>` |
| `verify` | `cnb materials verify` + `cnb pipeline profile validate` |
| `test *args` | 以 `E2E_PROFILE=<profile>` 调根 `test-e2e-core-all` |

项目特有 recipe 允许追加（如 bk 的 `smart`/`fixtures`），但不得覆盖上表语义。

## 三、禁止事项

- 不写业务逻辑（编排、下载、解包都在 cnb）
- 不写死镜像名（从 `pipeline plan` 推导）
- 不使用 `set dotenv-load`（配置项放 `.toml`，环境变量由调用方注入）
- 不引用 `script/fetch.sh` 等旧入口

## 四、自检

```bash
just --justfile project/<name>/justfile --list
just --justfile project/<name>/justfile --dry-run verify
```

两条都通过，且 recipe 与上表一致。

## 五、夹具与测试入口

每个子项目自带 `fixtures/`，并有 `just fixtures <recipe>` 转发：

| 项目类型 | 测试方式 | `just test` 行为 |
|---|---|---|
| paketo（lifecycle 直连） | `fixtures/justfile` 调根 `test-e2e-core*`，传 `E2E_PROFILE` + `E2E_FIXTURES` | 跑本目录全部夹具 |
| bk（bksmartpacker 契约） | `fixtures/justfile` 提供 `build/bps/pack/pack-all/inspect/layers` | 打包 s-mart 并校验 |

约定：
- `E2E_FIXTURES` 默认 `test/fixtures`，子项目通过自己的 `fixtures/justfile` 覆盖为本目录
- 夹具目录不入库 CI 依赖，只用于本地/发布前验证

## 六、层级陷阱

`project/<name>/fixtures/justfile` 的 `_root` 是 fixtures 目录，
仓库根在**三层之上**：

```make
_root := source_directory()
_repo := _root / "../../.."     # fixtures → <name> → project → 仓库根
```

`project/<name>/justfile` 则是两层（`_dir / ".." / ".."`）。
两处写错都会出现 `failed to read justfile at .../project/justfile` 这类错误。
