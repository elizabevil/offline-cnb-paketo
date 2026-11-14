// Package cnbconst 定义 CNB 与仓库共享的常量，避免各包重复声明。
package cnbconst

// 平台名（物料目录与 --platform 取值）。
const (
	PlatformAMD64 = "linux-amd64"
	PlatformARM64 = "linux-arm64"
	ArchPrefix    = "linux-"
)

// 阶段类型（profile.PlanStage.Kind）。
const (
	KindImage   = "image"
	KindCommand = "command"
	KindFetch   = "fetch"
)

// 命名构建上下文（Dockerfile --from=…）。
const (
	ContextBundle     = "bundle"
	ContextBuildpacks = "buildpacks"
)

// 仓库默认文件名与目录。
const (
	DefaultLockName = "assets.lock.toml"
	DefaultBundle   = "bundle"
	DefaultConfig   = "config"
)
