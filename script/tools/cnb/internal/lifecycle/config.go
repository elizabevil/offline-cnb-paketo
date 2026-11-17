package lifecycle

import "log/slog"

// Bins 指定四个 lifecycle 二进制的位置（由调用方决定）。
type Bins struct {
	Detector string
	Builder  string
	Analyzer string
	Exporter string
}

// Config 是一次 lifecycle 执行的全部输入。
//
// 字段按职责分组，避免"路径 / 导出 / 运行时 / 构建输入"混在一起：
//
//	Paths   目录布局
//	Bins    lifecycle 二进制
//	Build   构建输入（order、buildpack 过滤、源码来源）
//	Export  导出目标（镜像名、层包或 layout）
//	Runtime 运行时相关（Platform API、降权用户、容器运行时）
type Config struct {
	Paths   Paths
	Bins    Bins
	Build   BuildInput
	Export  ExportTarget
	Runtime RuntimeOptions

	// Log 是日志器（默认输出到 os.Stderr，级别 info）。
	Log *slog.Logger
	// DryRun 只打印将要执行的命令，不做任何落盘或执行。
	DryRun bool
}

// Paths 是本次执行的目录布局。
type Paths struct {
	App      string
	Layers   string
	Platform string
}

// BuildInput 是构建阶段的输入。
type BuildInput struct {
	// Order 是源 order.toml 路径；空则由调用方保证不启用过滤。
	Order string
	// Required 是 REQUIRED_BUILDPACKS 原始值。
	Required string
	// Aliases 把外部平台的 buildpack 名映射到 order 里的 id。
	Aliases map[string]string
	// SourceURL 是 SOURCE_GET_URL（仅支持 file://）。
	SourceURL string
}

// ExportTarget 是导出的目标。
type ExportTarget struct {
	// OutputImage 是应用镜像名。
	OutputImage string
	// RunImage 是 run 镜像引用。
	RunImage string
	// RunImageArchive 是 run image 的 docker-archive 路径（平台侧已落盘）。
	RunImageArchive string
	// CacheImage 是缓存镜像（可为空）。
	CacheImage string
	// Layout 为真时导出 OCI layout（跳过 app-layer 模式）。
	Layout bool
	// LayoutDir 是 layout 输出目录。
	LayoutDir string
	// LayerArchive 非空时只导出应用层到该 docker-archive。
	LayerArchive string
}

// RuntimeOptions 是运行时相关的选项。
type RuntimeOptions struct {
	// PlatformAPI 是 CNB Platform API 版本。
	PlatformAPI string
	// User 是 detect/build 使用的非 root 用户。
	User string
	// Runtime 是容器运行时命令（默认依次尝试 docker、podman）。
	Runtime string
}
