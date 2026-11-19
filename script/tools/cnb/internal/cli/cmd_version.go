package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"learn-cnb-paketo/tools/cnb/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "version information", Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.Get().String())
		},
	}
}
