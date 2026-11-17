package main

import (
	"github.com/spf13/cobra"
	"learn-cnb-paketo/tools/cnb/internal/lock"
)

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get <id|id@version> <platform>", Short: "输出 shell 赋值语句（供 fetch.sh eval）",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, _, _, err := open()
			if err != nil {
				return err
			}
			if err := lock.WriteShell(cmd.OutOrStdout(), l, args[0], lock.Platform(args[1])); err != nil {
				return &exitError{code: 2, msg: err.Error()}
			}
			return nil
		},
	}
}
