package main

import (
	"os"

	"learn-cnb-paketo/tools/cnb/internal/lifecycle"
)

// 适配层：把 bksmartpacker / BK 平台的环境变量映射为 lifecycle.Config。
// 通用用法应直接用 flags，或自行构造 Config。
const (
	envAppDir      = "CNB_APP_DIR"
	envLayersDir   = "CNB_LAYERS_DIR"
	envPlatformDir = "CNB_PLATFORM_DIR"
	envRunImage    = "CNB_RUN_IMAGE"
	envOutputImage = "OUTPUT_IMAGE"
	envCacheImage  = "CACHE_IMAGE"
	envSourceURL   = "SOURCE_GET_URL"
	envRequired    = "REQUIRED_BUILDPACKS"
	envPlatformAPI = "CNB_PLATFORM_API"
	envRunUser     = "CNB_RUN_USER"
	envLayoutDir   = "CNB_LAYOUT_DIR"
	envRunArchive  = "BKS_RUN_IMAGE_ARCHIVE"
	envExportMode  = "CNB_EXPORT_MODE"

	// lifecycle 二进制的默认位置（可在 flags 覆盖）。
	defaultLifecycleDir = "/cnb/lifecycle"
)

// defaultAliases 是 BK 名称到本项目 order id 的映射。
// 未命中时仍会按原名称匹配，因此非 BK 场景不受影响。
var defaultAliases = map[string]string{
	"bk-buildpack-go":     "paketo-buildpacks/go",
	"bk-buildpack-nodejs": "paketo-buildpacks/nodejs",
	"bk-buildpack-python": "paketo-buildpacks/python",
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// configFromEnv 返回带 BK 默认值的 Config。
func configFromEnv() lifecycle.Config {
	dir := defaultLifecycleDir
	return lifecycle.Config{
		Bins: lifecycle.Bins{
			Detector: dir + "/detector",
			Builder:  dir + "/builder",
			Analyzer: dir + "/analyzer",
			Exporter: dir + "/exporter",
		},
		Paths: lifecycle.Paths{
			App:      envOr(envAppDir, "/app"),
			Layers:   envOr(envLayersDir, "/layers"),
			Platform: envOr(envPlatformDir, "/platform"),
		},
		Build: lifecycle.BuildInput{
			Order:     "/cnb/order.toml",
			Required:  os.Getenv(envRequired),
			Aliases:   defaultAliases,
			SourceURL: os.Getenv(envSourceURL),
		},
		Export: lifecycle.ExportTarget{
			Mode:            lifecycle.ExportMode(envOr(envExportMode, string(lifecycle.ExportLayout))),
			OutputImage:     os.Getenv(envOutputImage),
			RunImage:        envOr(envRunImage, "fake-cnb-runner:latest"),
			RunImageArchive: os.Getenv(envRunArchive),
			CacheImage:      os.Getenv(envCacheImage),
			LayoutDir:       envOr(envLayoutDir, "/layers/oci"),
		},
		Runtime: lifecycle.RuntimeOptions{
			PlatformAPI: envOr(envPlatformAPI, "0.15"),
			User:        envOr(envRunUser, "cnb"),
		},
	}
}
