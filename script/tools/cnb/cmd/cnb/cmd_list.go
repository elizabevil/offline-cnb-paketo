package main

import (
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "物料清单（TSV：id version kind platforms）", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, _, _, err := open()
			if err != nil {
				return err
			}
			l.List(cmd.OutOrStdout())
			return nil
		},
	}
}
