package main

import (
	"os"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/lifecycle"
	"learn-cnb-paketo/tools/cnb/internal/render"
)

// newLifecycleCmd — 按 CNB / BK 环境变量执行一次构建（四阶段）
func newLifecycleCmd() *cobra.Command {
	parent := &cobra.Command{Use: "lifecycle", Short: "CNB lifecycle 执行器"}
	var (
		cfg          lifecycle.Config
		app          string
		layers       string
		platform     string
		output       string
		runImage     string
		cacheImage   string
		required     string
		dryRun       bool
		orderPath    string
		exportMode   string
		lifecycleDir string
		loglevel     string
	)
	run := &cobra.Command{
		Use:   "run",
		Short: "按环境变量执行 detect → build → analyze → export",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg = configFromEnv()
			if app != "" {
				cfg.Paths.App = app
			}
			if layers != "" {
				cfg.Paths.Layers = layers
			}
			if platform != "" {
				cfg.Paths.Platform = platform
			}
			if output != "" {
				cfg.Export.OutputImage = output
			}
			if runImage != "" {
				cfg.Export.RunImage = runImage
			}
			if cacheImage != "" {
				cfg.Export.CacheImage = cacheImage
			}
			if required != "" {
				cfg.Build.Required = required
			}
			if orderPath != "" {
				cfg.Build.Order = orderPath
			}
			if exportMode != "" {
				cfg.Export.Mode = lifecycle.ExportMode(exportMode)
			}
			if lifecycleDir != "" {
				cfg.Bins = lifecycle.Bins{
					Detector: lifecycleDir + "/detector",
					Builder:  lifecycleDir + "/builder",
					Analyzer: lifecycleDir + "/analyzer",
					Exporter: lifecycleDir + "/exporter",
				}
			}
			cfg.DryRun = dryRun

			if err := cfg.Prepare(); err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			cfg.Log = render.NewLogger(os.Stderr, loglevel)
			if err := cfg.Run(); err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			return nil
		},
	}
	run.Flags().StringVar(&app, "app", "", "应用目录（默认 CNB_APP_DIR）")
	run.Flags().StringVar(&layers, "layers", "", "层目录（默认 CNB_LAYERS_DIR）")
	run.Flags().StringVar(&platform, "platform", "", "平台目录（默认 CNB_PLATFORM_DIR）")
	run.Flags().StringVar(&output, "output-image", "", "导出镜像（默认 OUTPUT_IMAGE）")
	run.Flags().StringVar(&runImage, "run-image", "", "run 镜像（默认 CNB_RUN_IMAGE）")
	run.Flags().StringVar(&cacheImage, "cache-image", "", "缓存镜像（默认 CACHE_IMAGE）")
	run.Flags().StringVar(&required, "required-buildpacks", "", "限定 buildpack（默认 REQUIRED_BUILDPACKS）")
	run.Flags().StringVar(&lifecycleDir, "lifecycle-dir", "", "lifecycle 二进制目录（默认 /cnb/lifecycle）")
	run.Flags().StringVar(&loglevel, "log-level", "info", "log level: debug|info|warn|error")
	run.Flags().StringVar(&orderPath, "order", "", "order.toml 路径（默认 /cnb/order.toml）")
	run.Flags().StringVar(&exportMode, "export", "", "导出方式：daemon | layout | none（默认取环境：BK→daemon，其余 layout）")
	run.Flags().BoolVar(&dryRun, "dry-run", false, "只打印四阶段命令")
	parent.AddCommand(run)
	return parent
}
