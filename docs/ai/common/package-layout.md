# 包分层与合并原则

适用于 `script/tools/cnb/internal`。目的是避免包越拆越碎、职责模糊。

## 一、三层结构

| 层 | 包 | 允许依赖 |
|---|---|---|
| 协议 | `spec/*`（manifest、profile、lifecycle、platform、imagefmt） | 仅标准库、`errs`、`spec/warn` |
| 基础设施 | `errs` `fsutil` `strutil` `timefmt` `tomledit` `user` `version` `workspace` `render` | 仅标准库与同层 |
| 领域模型 | `lock` `materials` `profile` `checks` | 基础设施 + 同层 |
| 执行 | `lifecycle` `pipeline` `archive` `repack` | 以上全部 |
| 入口 | `cmd/cnb` | 任意 internal |

依赖方向只能自上而下：入口 → 执行 → 领域 → 基础设施 → 协议。
**协议包不得依赖任何业务包**；业务包只能消费协议包的结构体，禁止二次解析。
协议包开发规范见 [../pipeline/protocol-spec.md](../pipeline/protocol-spec.md)。

## 二、什么时候该新建包

同时满足两条才建：

1. 有独立、能用一句话说清的职责（例如「物料清单的解析与校验」）；
2. 至少有两个以上消费方，或该职责本身有内部状态/多条流程。

只有单个消费方、又只是几行辅助函数 → 放回消费方所在包。

## 三、什么时候该合并

| 信号 | 处理 |
|---|---|
| 包内只有一个文件且 < 100 行 | 并入职责相同的包 |
| 文件 < 80 行、内容只是辅助函数 | 并入同包主文件 |
| 两包共用同一批常量/路径规则 | 常量归到领域包，另一包 import |
| 两包读写同一份磁盘状态 | 合并（例：`bundle` + `materials`） |
| 两包都是"检查/校验" | 合并（例：`bpcheck`+`dockerfile`+`doctor`+`buildmeta` → `checks`） |
| 两包都是"归档读写" | 合并（例：`apparchive` + `archive`） |

合并时先处理重名符号（按域加前缀，如 `BuildpackReport` / `DockerfileReport`），再删旧目录。

## 四、拆分粒度

- 文件按**职责**拆，不按行数硬切。一个文件应能用一个短语概括（`validate.go`、`args.go`）。
- 一个包里同类文件控制在 5~8 个以内；超过说明可以再分层。
- 禁止出现 `util.go` / `common.go` / `misc.go` 这类无意义文件名；辅助函数放回它被使用的地方。

## 五、命名约定

| 场景 | 约定 | 例子 |
|---|---|---|
| 有副作用（写文件/写标准输出） | `Write*` | `lock.WriteList` |
| 纯计算返回数据 | 名词或 `Get*` | `plan.Summary` |
| 判断 | `Is*` / `Has*` | `Platform.IsLinux` |
| 构造 | `New*` | `render.NewLogger` |
| 协议枚举 | 自定义类型 + 常量 | `lock.Kind`、`profile.StageKind`、`materials.Action` |

## 六、协议常量归属

协议的解析器归属见 [协议分层](../../design/protocol-layers.md)：每个协议只有一个解析器，统一放在 `internal/spec/*`。

- 清单相关（平台、解包方式、段落名、默认文件名）→ `lock`
- 计划相关（阶段类型）→ `profile`
- 同步动作 → `materials`
- Dockerfile/元数据检查 → `checks`

同一概念只允许一处定义；外部格式（OCI、docker-archive、builder metadata）的 JSON 键名例外，见 `docs/ai/pipeline/lifecycle-contract.md` 与 `internal/repack` 注释。

## 七、提交前自检

- [ ] 新包满足「独立职责 + 至少两个消费方」
- [ ] 没有 < 80 行的纯辅助文件游离在外
- [ ] 没有反向依赖（领域包 import 执行包）
- [ ] 常量只有一个来源
- [ ] `just check`（fmt / vet / lint / test / tags）全绿

## 八、当前快照（17 个包）

```
archive checks errs fsutil lifecycle lock materials pipeline
profile render repack strutil timefmt tomledit user version workspace
```

历史合并记录：25 → 23（bundle/apparchive）→ 20（检查类）→ 17（基础设施类）。
