package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"learn-cnb-paketo/tools/cnb/internal/lock"
)

func newLockCmd() *cobra.Command {
	parent := &cobra.Command{Use: "lock", Short: "lock 维护"}
	parent.AddCommand(&cobra.Command{
		Use: "set-sha <ref> <platform> <sha256>", Short: "写入/校验某平台哈希", Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := workspaceLockPath(opts.lockPath)
			if err != nil {
				return err
			}
			changed, err := lock.SetSHA(path, args[0], args[1], args[2])
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			if changed {
				fmt.Fprintf(cmd.OutOrStdout(), "  + 已固定 %s[%s]\n", args[0], args[1])
			}
			return nil
		},
	})
	return parent
}
