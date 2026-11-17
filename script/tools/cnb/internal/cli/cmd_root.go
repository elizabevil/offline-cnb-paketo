package cli

import (
	"os"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/version"
)

func newRootCmd() *cobra.Command {
	logLevel := "info"
	root := &cobra.Command{
		Use:   "cnb",
		Short: "基座小工具：materials / pipeline / image / lifecycle",
		Long: "cnb 读取 assets.lock.toml 与 config/projects/*.toml，\n" +
			"提供物料解析、流水线渲染与 SmartPackage 规范化能力。",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Get().Short(),
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			// 容器内（如 lifecycle 执行器）没有仓库标记，不应因此失败；
			// 需要仓库根的子命令会自行报错。
			setupLogger(logLevel)
			if root, err := resolveRepo(); err == nil {
				_ = os.Setenv("CNB_REPO_ROOT", root)
			}
			return nil
		},
	}
	root.SetVersionTemplate("cnb {{.Version}}\n")

	pf := root.PersistentFlags()
	pf.StringVar(&logLevel, "log-level", "info", "log level: debug|info|warn|error")
	pf.StringVar(&opts.lockPath, "file", "", "lock 文件路径（默认 <exe>/../../assets.lock.toml 或 LOCK_FILE）")
	pf.StringVar(&opts.bundle, "bundle", "", "物料目录（默认 <lock 所在目录>/bundle）")
	pf.StringVar(&opts.platform, "platform", "", "目标平台：linux-amd64 | linux-arm64")
	pf.BoolVar(&opts.strict, "strict", false, "verify 时未固定 sha256 也算失败")
	pf.BoolVar(&opts.quiet, "quiet", false, "verify 仅输出失败项")
	pf.StringVar(&opts.repo, "repo", "", "仓库根（默认 $CNB_REPO_ROOT 或由可执行文件位置推导）")
	pf.StringVar(&opts.configDir, "config-dir", "config", "配置目录（相对仓库根）")

	root.AddCommand(
		newMaterialsCmd(),    // 物料域
		newPipelineCmd(),     // 流水线域
		newBuildpackCmd(),    // buildpack 域
		newLifecycleCmd(),    // CNB 执行器
		newImageCmd(),        // 镜像格式转换
		newPipelineLintCmd(), // 流水线 lint
		newVersionCmd(),
	)
	return root
}
