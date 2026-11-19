package cli

import (
	"os"

	"learn-cnb-paketo/tools/cnb/internal/lifecycle"
	"learn-cnb-paketo/tools/cnb/internal/spec/platform"
)

// Adapter: maps the platform env contract (platform.Env) to lifecycle.Config.
// cnb stays platform agnostic; env names and defaults live in spec/platform.
const defaultLifecycleDir = "/cnb/lifecycle"

// configFromEnv builds a lifecycle config from the platform contract.
func configFromEnv() (lifecycle.Config, error) {
	env, warns, err := platform.Parse(os.Getenv)
	if err != nil {
		return lifecycle.Config{}, err
	}
	for _, w := range warns {
		logr.Warn("platform env", "warning", w.String())
	}
	dir := defaultLifecycleDir
	return lifecycle.Config{
		Bins: lifecycle.Bins{
			Detector: dir + "/detector",
			Builder:  dir + "/builder",
			Analyzer: dir + "/analyzer",
			Exporter: dir + "/exporter",
		},
		Paths: lifecycle.Paths{
			App:      env.AppDir,
			Layers:   env.LayersDir,
			Platform: env.PlatformDir,
		},
		Build: lifecycle.BuildInput{
			Order:     "/cnb/order.toml",
			Required:  joinRequired(env.Required),
			SourceURL: env.SourceURL,
			Aliases:   env.BuildpackAliases,
		},
		Export: lifecycle.ExportTarget{
			Mode:            lifecycle.ExportMode(env.ExportMode),
			OutputImage:     env.OutputImage,
			RunImage:        env.RunImage,
			RunImageArchive: env.RunImageArchive,
			CacheImage:      env.CacheImage,
			LayoutDir:       env.LayoutDir,
			OutputArchive:   env.LayerArchive,
		},
		Runtime: lifecycle.RuntimeOptions{
			PlatformAPI: env.PlatformAPI,
			User:        envUser(),
		},
	}, nil
}

// joinRequired restores split required entries to the raw lifecycle string.
func joinRequired(items []string) string {
	out := ""
	for i, it := range items {
		if i > 0 {
			out += ";"
		}
		out += it
	}
	return out
}

// envUser returns the user running detect/build (default cnb).
func envUser() string {
	if u := os.Getenv("CNB_RUN_USER"); u != "" {
		return u
	}
	return "cnb"
}
