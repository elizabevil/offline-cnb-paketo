package main

import (
	"github.com/spf13/cobra"
	"learn-cnb-paketo/tools/cnb/internal/lock"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "物料清单（TSV：id version kind platforms）", Args: cobra.NoArgs,
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
