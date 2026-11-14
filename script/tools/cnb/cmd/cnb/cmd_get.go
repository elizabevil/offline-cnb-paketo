package main

import (
	"github.com/spf13/cobra"
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
			if err := l.WriteShell(cmd.OutOrStdout(), args[0], args[1]); err != nil {
				return &exitError{code: 2, msg: err.Error()}
			}
			return nil
		},
	}
}
