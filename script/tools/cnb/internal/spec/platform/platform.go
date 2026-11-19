// Package platform 定义平台（CNB / SmartPacker）注入的环境契约。
package platform

import (
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
)

// 环境变量名（唯一来源）。
const (
	EnvSourceURL        = "SOURCE_GET_URL"
	EnvRequired         = "REQUIRED_BUILDPACKS"
	EnvOutputImage      = "OUTPUT_IMAGE"
	EnvRunImage         = "CNB_RUN_IMAGE"
	EnvCacheImage       = "CACHE_IMAGE"
	EnvAppDir           = "CNB_APP_DIR"
	EnvLayersDir        = "CNB_LAYERS_DIR"
	EnvPlatformDir      = "CNB_PLATFORM_DIR"
	EnvPlatformAPI      = "CNB_PLATFORM_API"
	EnvExportMode       = "CNB_EXPORT_MODE"
	EnvRunArchive       = "CNB_RUN_IMAGE_ARCHIVE"
	EnvLayoutDir        = "CNB_LAYOUT_DIR"
	EnvBuildpackAliases = "CNB_BUILDPACK_ALIASES"
	EnvLayerArchive     = "CNB_LAYER_ARCHIVE"
)

// 默认值。
const (
	DefaultAppDir      = "/app"
	DefaultLayersDir   = "/layers"
	DefaultPlatformDir = "/platform"
	DefaultRunImage    = "fake-cnb-runner:latest"
	DefaultPlatformAPI = "0.15"
	DefaultLayoutDir   = "/layers/oci"
	// DefaultTargetPlatform is the default target platform for profile init.
	DefaultTargetPlatform = manifest.PlatformAMD64
)

// ExportMode 是导出方式。
type ExportMode string

const (
	// ExportDaemon 写进容器运行时。
	ExportDaemon ExportMode = "daemon"
	// ExportLayout 写 OCI layout 目录。
	ExportLayout ExportMode = "layout"
	// ExportNone 只构建不导出。
	ExportNone ExportMode = "none"
)

// Env 是平台契约的结构化表示。
type Env struct {
	SourceURL       string
	Required        []string
	OutputImage     string
	RunImage        string
	CacheImage      string
	AppDir          string
	LayersDir       string
	PlatformDir     string
	PlatformAPI     string
	ExportMode      ExportMode
	RunImageArchive string
	LayoutDir       string
	// BuildpackAliases 把平台侧 buildpack 名映射到 order 里的 id
	// （格式 "a=b,c=d"，由调用方注入；cnb 不内置任何平台名称）。
	BuildpackAliases map[string]string
	// LayerArchive 非空时，layout 导出后把目录打包为该路径（OCI archive）。
	LayerArchive string
}

// Parse 读取环境变量并填默认值。
func Parse(getenv func(string) string) (Env, []errs.Warning, error) {
	env := Env{
		SourceURL:        getenv(EnvSourceURL),
		Required:         splitList(getenv(EnvRequired)),
		OutputImage:      getenv(EnvOutputImage),
		RunImage:         valueOr(getenv(EnvRunImage), DefaultRunImage),
		CacheImage:       getenv(EnvCacheImage),
		AppDir:           valueOr(getenv(EnvAppDir), DefaultAppDir),
		LayersDir:        valueOr(getenv(EnvLayersDir), DefaultLayersDir),
		PlatformDir:      valueOr(getenv(EnvPlatformDir), DefaultPlatformDir),
		PlatformAPI:      valueOr(getenv(EnvPlatformAPI), DefaultPlatformAPI),
		ExportMode:       ExportMode(valueOr(getenv(EnvExportMode), string(ExportLayout))),
		RunImageArchive:  getenv(EnvRunArchive),
		LayoutDir:        valueOr(getenv(EnvLayoutDir), DefaultLayoutDir),
		BuildpackAliases: parseAliases(getenv(EnvBuildpackAliases)),
		LayerArchive:     getenv(EnvLayerArchive),
	}
	if err := env.Validate(); err != nil {
		return Env{}, nil, err
	}
	return env, nil, nil
}

// Validate 校验取值组合。
func (e Env) Validate() error {
	switch e.ExportMode {
	case ExportDaemon:
		if e.RunImage == "" {
			return &errs.Missing{What: "run image (export mode daemon)"}
		}
	case ExportLayout:
		if e.LayoutDir == "" {
			return &errs.Missing{What: "layout dir (export mode layout)"}
		}
	case ExportNone:
	default:
		return &errs.Invalid{Field: "export mode", Value: string(e.ExportMode), Reason: "expect daemon | layout | none"}
	}
	return nil
}

// SourcePath 返回 SOURCE_GET_URL 中的本地路径；非 file:// 时返回空。
func (e Env) SourcePath() string {
	if after, ok := strings.CutPrefix(e.SourceURL, "file://"); ok {
		return after
	}
	return ""
}

// parseAliases 解析 "a=b,c=d" 形式的别名表。
func parseAliases(s string) map[string]string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	out := map[string]string{}
	for pair := range strings.SplitSeq(s, ",") {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

// splitList 按空白切分（REQUIRED_BUILDPACKS 是以 ; 分隔的条目，条目内是空格分列）。
func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for entry := range strings.SplitSeq(s, ";") {
		if f := strings.TrimSpace(entry); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// valueOr 返回非空值或默认值。
func valueOr(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
