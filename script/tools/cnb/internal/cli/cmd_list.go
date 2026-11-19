package cli

import (
	"learn-cnb-paketo/tools/cnb/internal/lock"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "list materials (TSV: id version kind platforms)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, _, _, err := open()
			if err != nil {
				return err
			}
			lock.WriteList(cmd.OutOrStdout(), l)
			return nil
		},
	}
}
