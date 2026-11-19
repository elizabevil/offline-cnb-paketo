package cli

import (
	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/lifecycle"
)

// lifecycleFlags holds every flag of `cnb lifecycle run`.
// Environment variables provide defaults; flags override them; parsing lives here.
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

// bindLifecycleFlags registers and binds the flags.
func bindLifecycleFlags(cmd *cobra.Command) *lifecycleFlags {
	f := &lifecycleFlags{}
	fl := cmd.Flags()
	fl.StringVar(&f.app, "app", "", "app directory (default CNB_APP_DIR)")
	fl.StringVar(&f.layers, "layers", "", "layers directory (default CNB_LAYERS_DIR)")
	fl.StringVar(&f.platform, "platform", "", "platform directory (default CNB_PLATFORM_DIR)")
	fl.StringVar(&f.outputImage, "output-image", "", "output image (default OUTPUT_IMAGE)")
	fl.StringVar(&f.runImage, "run-image", "", "run image (default CNB_RUN_IMAGE)")
	fl.StringVar(&f.cacheImage, "cache-image", "", "cache image (default CACHE_IMAGE)")
	fl.StringVar(&f.required, "required-buildpacks", "", "restrict buildpacks (default REQUIRED_BUILDPACKS)")
	fl.StringVar(&f.orderPath, "order", "", "path to order.toml (default /cnb/order.toml)")
	fl.StringVar(&f.exportMode, "export", "", "export mode: daemon | layout | none (default CNB_EXPORT_MODE, falling back to layout)")
	fl.StringVar(&f.lifecycleDir, "lifecycle-dir", "", "lifecycle binary directory (default /cnb/lifecycle)")
	fl.StringVar(&f.logLevel, "log-level", "info", "log level: debug|info|warn|error")
	fl.BoolVar(&f.dryRun, "dry-run", false, "print the four phase commands only")
	return f
}

// resolve merges environment defaults and flags into a Config.
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
