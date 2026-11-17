package main

import (
	"os"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/render"
)

// newLifecycleCmd — 按环境变量与 flags 执行一次构建（四阶段）。
// 参数解析集中在 lifecycle_flags.go；环境变量默认值集中在 lifecycle_env.go。
func newLifecycleCmd() *cobra.Command {
	parent := &cobra.Command{Use: "lifecycle", Short: "CNB lifecycle 执行器"}
	run := &cobra.Command{
		Use:   "run",
		Short: "执行 detect → build → analyze → export",
	}
	flags := bindLifecycleFlags(run)
	run.RunE = func(*cobra.Command, []string) error {
		cfg := flags.resolve()
		cfg.Log = render.NewLogger(os.Stderr, flags.logLevel)
		if err := cfg.Prepare(); err != nil {
			return &exitError{code: ExitFailure, msg: err.Error()}
		}
		if err := cfg.Run(); err != nil {
			return &exitError{code: ExitFailure, msg: err.Error()}
		}
		return nil
	}
	parent.AddCommand(run)
	return parent
}
