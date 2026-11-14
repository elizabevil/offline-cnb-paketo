package lock

// 平台与目录约定（lock 作用域）。
const (

	// DefaultBundleDir 是 lock 同级的物料目录名。
	DefaultBundleDir = DefaultBundle

	// LayoutArchFirst 是 bundle/<platform>/... 布局。
	LayoutArchFirst = "arch-first"
	// LayoutKindFirst 是 bundle/binaries/<platform>/... 布局。
	LayoutKindFirst = "kind-first"
)
