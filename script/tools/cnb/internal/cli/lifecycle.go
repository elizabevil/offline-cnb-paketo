package cli

import (
	"os"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/render"
)

// newLifecycleCmd runs one build (four phases) from env and flags.
// Flag parsing lives in lifecycle_flags.go; env defaults in lifecycle_env.go.
func newLifecycleCmd() *cobra.Command {
	parent := &cobra.Command{Use: "lifecycle", Short: "CNB lifecycle runner"}
	run := &cobra.Command{
		Use:   "run",
		Short: "run detect → build → analyze → export",
	}
	flags := bindLifecycleFlags(run)
	run.RunE = func(*cobra.Command, []string) error {
		cfg, err := flags.resolve()
		if err != nil {
			return failUsage(err)
		}
		cfg.Log = render.NewLogger(os.Stderr, flags.logLevel)
		if err := cfg.Prepare(); err != nil {
			return failExecution(err)
		}
		if err := cfg.Run(); err != nil {
			return failExecution(err)
		}
		return nil
	}
	parent.AddCommand(run)
	return parent
}
