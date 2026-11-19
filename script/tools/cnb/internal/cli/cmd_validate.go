package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use: "validate", Short: "validate lock structure", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, _, _, err := open()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			problems := l.Validate()
			for _, p := range problems {
				fmt.Fprintf(out, "  ✗ %s\n", p)
			}
			if len(problems) > 0 {
				return &exitError{code: ExitFailure}
			}
			fmt.Fprintf(out, "  ✓ lock structure ok: %d materials, %d platforms\n",
				len(l.Assets), len(l.Platforms()))
			return nil
		},
	}
}
