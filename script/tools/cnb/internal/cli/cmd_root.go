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
		Short: "foundation tooling: materials / pipeline / image / lifecycle",
		Long: "cnb reads assets.lock.toml and config/projects/*.toml,\n" +
			"providing material resolution, pipeline rendering, and SmartPackage normalization.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Get().Short(),
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			// Inside containers (e.g. the lifecycle runner) there is no repo marker; do not fail here.
			// Subcommands that need the repo root report their own errors.
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
	pf.StringVar(&opts.lockPath, "file", "", "lock file path (default <exe>/../../assets.lock.toml or LOCK_FILE)")
	pf.StringVar(&opts.bundle, "bundle", "", "material directory (default <lock dir>/bundle)")
	pf.StringVar(&opts.platform, "platform", "", "target platform: linux-amd64 | linux-arm64")
	pf.BoolVar(&opts.strict, "strict", false, "treat unpinned sha256 as an error during verify")
	pf.BoolVar(&opts.quiet, "quiet", false, "verify prints failures only")
	pf.StringVar(&opts.repo, "repo", "", "repo root (default $CNB_REPO_ROOT or derived from the executable path)")
	pf.StringVar(&opts.configDir, "config-dir", "config", "config directory (relative to the repo root)")

	root.AddCommand(
		newInitCmd(),         // scaffold a project
		newMaterialsCmd(),    // materials
		newPipelineCmd(),     // pipeline
		newBuildpackCmd(),    // buildpacks
		newLifecycleCmd(),    // CNB runner
		newImageCmd(),        // image format conversion
		newPipelineLintCmd(), // pipeline lint
		newVersionCmd(),
	)
	return root
}
