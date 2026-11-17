package main

import (
	"os"

	"learn-cnb-paketo/tools/cnb/internal/lifecycle"
	"learn-cnb-paketo/tools/cnb/internal/spec/platform"
)

// 适配层：把平台环境契约（platform.Env）映射为 lifecycle.Config。
// cnb 不感知具体平台；环境变量名与默认值集中在 spec/platform。
const defaultLifecycleDir = "/cnb/lifecycle"

// configFromEnv 由平台契约生成 lifecycle 配置。
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
		},
		Export: lifecycle.ExportTarget{
			Mode:            lifecycle.ExportMode(env.ExportMode),
			OutputImage:     env.OutputImage,
			RunImage:        env.RunImage,
			RunImageArchive: env.RunImageArchive,
			CacheImage:      env.CacheImage,
			LayoutDir:       env.LayoutDir,
		},
		Runtime: lifecycle.RuntimeOptions{
			PlatformAPI: env.PlatformAPI,
			User:        envUser(),
		},
	}, nil
}

// joinRequired 把已切分的 required 条目还原为 lifecycle 期望的原始串。
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

// envUser 返回运行 detect/build 的用户（默认 cnb）。
func envUser() string {
	if u := os.Getenv("CNB_RUN_USER"); u != "" {
		return u
	}
	return "cnb"
}
