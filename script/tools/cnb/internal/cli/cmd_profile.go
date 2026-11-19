package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/render"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// newProfileCmd covers pipeline config: generate, validate, show.
func newProfileCmd() *cobra.Command {
	parent := &cobra.Command{Use: "profile", Short: "pipeline config: validate / show"}
	parent.AddCommand(newProfileValidateCmd())
	var showOpts profileShowOptions
	showCmd := &cobra.Command{
		Use: "show <file>", Short: "print the config as structured data", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, _, err := loadProfileFile(args[0])
			if err != nil {
				return err
			}
			out, err := render.FormatStruct(f, showOpts.format)
			if err != nil {
				return failUsage(err)
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	showCmd.Flags().StringVar(&showOpts.format, "format", "yaml", "output format: yaml | json")
	parent.AddCommand(showCmd)

	return parent
}

// profileShowOptions holds profile show flags.
type profileShowOptions struct {
	format string
}

// newProfileValidateCmd validates a pipeline config.
func newProfileValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use: "validate <file>", Short: "validate a pipeline config", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, warns, err := loadProfileFile(args[0])
			if err != nil {
				return err
			}
			failed := 0
			for _, w := range warns {
				fmt.Fprintf(cmd.OutOrStdout(), "  ! %s\n", w.String())
			}
			for _, e := range f.Validate() {
				failed++
				fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %v\n", e)
			}
			if failed > 0 {
				return &exitError{code: ExitFailure}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ valid pipeline: %s (%d materials, %d stages, platform %s)\n",
				f.Metadata.Name, len(f.Spec.Assets), len(f.Spec.Stages), f.Spec.Project.Platform)
			return nil
		},
	}
}

// assertWorkspace keeps the workspace import in use.
var _ = workspace.Open
