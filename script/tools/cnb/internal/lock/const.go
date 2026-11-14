package lock

// 平台与目录约定（lock 作用域）。
const (
	// PlatformAMD64 是 linux/amd64 物料目录名。
	PlatformAMD64 = "linux-amd64"
	// PlatformARM64 是 linux/arm64 物料目录名。
	PlatformARM64 = "linux-arm64"

	// DefaultBundleDir 是 lock 同级的物料目录名。
	DefaultBundleDir = "bundle"

	// LayoutArchFirst 是 bundle/<platform>/... 布局。
	LayoutArchFirst = "arch-first"
	// LayoutKindFirst 是 bundle/binaries/<platform>/... 布局。
	LayoutKindFirst = "kind-first"
)
