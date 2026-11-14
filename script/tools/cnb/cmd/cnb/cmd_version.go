package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"learn-cnb-paketo/tools/cnb/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "版本信息", Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.Get().String())
		},
	}
}
