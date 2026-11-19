package manifest

// CNB 与仓库共享的常量（原 internal/cnbconst）。
const (
	// PlatformAMD64 / PlatformARM64 是物料目录与 --platform 的取值。
	PlatformAMD64 = "linux-amd64"
	PlatformARM64 = "linux-arm64"
	// ArchPrefix 是平台名中的架构前缀。
	ArchPrefix = "linux-"

	// KindImage / KindCommand / KindFetch 是阶段类型。
	KindImage   = "image"
	KindCommand = "command"
	KindFetch   = "fetch"

	// ContextBundle / ContextBuildpacks 是命名构建上下文。
	ContextBundle     = "bundle"
	ContextBuildpacks = "buildpacks"

	// DefaultStorageLayout 是物料目录的默认布局。
	DefaultStorageLayout = "arch-first"

	// DefaultLockName / DefaultBundle / DefaultConfig 是仓库默认名。
	DefaultManifestName = "assets.toml"
	DefaultLockName     = "assets.lock.toml"
	DefaultBundle       = "bundle"
	DefaultConfig       = "config"
)
