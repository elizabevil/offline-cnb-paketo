package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

func newPlatformsCmd() *cobra.Command {
	return &cobra.Command{
		Use: "platforms", Short: "平台矩阵（逗号分隔）", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, _, _, err := open()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(l.Platforms(), ","))
			return nil
		},
	}
}
