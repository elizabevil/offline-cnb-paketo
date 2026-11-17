package main

import (
	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/lifecycle"
)

// lifecycleFlags 是 `cnb lifecycle run` 的全部命令行参数。
// 环境变量只是默认值，flags 覆盖它们；解析集中在本文件。
type lifecycleFlags struct {
	app          string
	layers       string
	platform     string
	outputImage  string
	runImage     string
	cacheImage   string
	required     string
	orderPath    string
	exportMode   string
	lifecycleDir string
	logLevel     string
	dryRun       bool
}

// bindLifecycleFlags 注册并绑定参数。
func bindLifecycleFlags(cmd *cobra.Command) *lifecycleFlags {
	f := &lifecycleFlags{}
	fl := cmd.Flags()
	fl.StringVar(&f.app, "app", "", "应用目录（默认 CNB_APP_DIR）")
	fl.StringVar(&f.layers, "layers", "", "层目录（默认 CNB_LAYERS_DIR）")
	fl.StringVar(&f.platform, "platform", "", "平台目录（默认 CNB_PLATFORM_DIR）")
	fl.StringVar(&f.outputImage, "output-image", "", "导出镜像（默认 OUTPUT_IMAGE）")
	fl.StringVar(&f.runImage, "run-image", "", "run 镜像（默认 CNB_RUN_IMAGE）")
	fl.StringVar(&f.cacheImage, "cache-image", "", "缓存镜像（默认 CACHE_IMAGE）")
	fl.StringVar(&f.required, "required-buildpacks", "", "限定 buildpack（默认 REQUIRED_BUILDPACKS）")
	fl.StringVar(&f.orderPath, "order", "", "order.toml 路径（默认 /cnb/order.toml）")
	fl.StringVar(&f.exportMode, "export", "", "导出方式：daemon | layout | none（默认 CNB_EXPORT_MODE，缺省 layout）")
	fl.StringVar(&f.lifecycleDir, "lifecycle-dir", "", "lifecycle 二进制目录（默认 /cnb/lifecycle）")
	fl.StringVar(&f.logLevel, "log-level", "info", "log level: debug|info|warn|error")
	fl.BoolVar(&f.dryRun, "dry-run", false, "只打印四阶段命令")
	return f
}

// resolve 把环境变量默认值与 flags 合并成一份 Config。
func (f lifecycleFlags) resolve() (lifecycle.Config, error) {
	cfg, err := configFromEnv()
	if err != nil {
		return lifecycle.Config{}, err
	}
	if f.app != "" {
		cfg.Paths.App = f.app
	}
	if f.layers != "" {
		cfg.Paths.Layers = f.layers
	}
	if f.platform != "" {
		cfg.Paths.Platform = f.platform
	}
	if f.outputImage != "" {
		cfg.Export.OutputImage = f.outputImage
	}
	if f.runImage != "" {
		cfg.Export.RunImage = f.runImage
	}
	if f.cacheImage != "" {
		cfg.Export.CacheImage = f.cacheImage
	}
	if f.exportMode != "" {
		cfg.Export.Mode = lifecycle.ExportMode(f.exportMode)
	}
	if f.required != "" {
		cfg.Build.Required = f.required
	}
	if f.orderPath != "" {
		cfg.Build.Order = f.orderPath
	}
	if f.lifecycleDir != "" {
		cfg.Bins = lifecycle.Bins{
			Detector: f.lifecycleDir + "/detector",
			Builder:  f.lifecycleDir + "/builder",
			Analyzer: f.lifecycleDir + "/analyzer",
			Exporter: f.lifecycleDir + "/exporter",
		}
	}
	cfg.DryRun = f.dryRun
	return cfg, nil
}
