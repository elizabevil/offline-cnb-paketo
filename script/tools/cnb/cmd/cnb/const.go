package main

// CLI 作用域常量。
const (
	// DefaultLockName 是默认物料清单文件名。
	DefaultLockName = "assets.lock.toml"
	// DefaultConfigDir 是默认流程配置目录。
	DefaultConfigDir = "config/projects"

	// ExitOK 表示成功。
	ExitOK = 0
	// ExitFailure 表示执行失败。
	ExitFailure = 1
	// ExitUsage 表示参数或用法错误。
	ExitUsage = 2

	// BundleContext 是 Dockerfile 引用物料的命名构建上下文。
	BundleContext = "bundle"
	// BuildpacksContext 是引用 buildpack 的命名构建上下文。
	BuildpacksContext = "buildpacks"
)
