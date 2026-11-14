package lifecycle

// 生命周期阶段与路径常量。
const (
	// 应用与工作目录默认值。
	DefaultAppDir      = "/app"
	DefaultLayersDir   = "/layers"
	DefaultPlatformDir = "/platform"
	DefaultOrderPath   = "/cnb/order.toml"

	// 输出与缓存。
	DefaultLayoutDir   = "/layers/oci"
	DefaultRunImage    = "fake-cnb-runner:latest"
	DefaultPlatformAPI = "0.15"
	DefaultRunUser     = "cnb"

	// lifecycle 二进制路径。
	BinDetector = "/cnb/lifecycle/detector"
	BinBuilder  = "/cnb/lifecycle/builder"
	BinAnalyzer = "/cnb/lifecycle/analyzer"
	BinExporter = "/cnb/lifecycle/exporter"

	// OrderFileName 是过滤后 order 的文件名。
	OrderFileName = "order.toml"

	// UserDatabase 是用户信息文件。
	UserDatabase = "/etc/passwd"

	// EchoTag 是错误信息前缀，用于区分阶段输出。
	PhaseDetect  = "detect"
	PhaseBuild   = "build"
	PhaseAnalyze = "analyze"
	PhaseExport  = "export"
)
