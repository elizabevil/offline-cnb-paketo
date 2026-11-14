// Package lifecycle 执行 CNB 四阶段（detect/build/analyze/export）。
// 本包不假设任何目录布局：路径、二进制位置、平台与用户全部由调用方给出。
package lifecycle

// 阶段名与文件约定（与 CNB 规范一致，不是本项目特有）。
const (
	PhaseDetect  = "detect"
	PhaseBuild   = "build"
	PhaseAnalyze = "analyze"
	PhaseExport  = "export"

	// OrderFileName 是 lifecycle 读取的 order 文件名。
	OrderFileName = "order.toml"

	// UserDatabase 是查询用户 uid/gid 的文件。
	UserDatabase = "/etc/passwd"

	// 传给 lifecycle/buildpack 的环境变量名。
	envPlatformAPI  = "CNB_PLATFORM_API"
	envAppDir       = "CNB_APP_DIR"
	envLayersDir    = "CNB_LAYERS_DIR"
	envPlatformDir  = "CNB_PLATFORM_DIR"
	envExperimental = "CNB_EXPERIMENTAL_MODE"

	// FileURLScheme 是本地源码的 URL 前缀。
	FileURLScheme = "file://"
)
