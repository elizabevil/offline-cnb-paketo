package main

import (
	"github.com/spf13/cobra"
	"learn-cnb-paketo/tools/cnb/internal/materials"
)

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use: "verify [ids...]", Short: "校验归档哈希、解压标记与预期产物",
		RunE: func(_ *cobra.Command, ids []string) error {
			l, platform, bundleDir, err := open()
			if err != nil {
				return err
			}
			rep, err := materials.Check(l, bundleDir, platform, ids)
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			if !printReport(rep, opts.strict, opts.quiet) {
				return &exitError{code: 1}
			}
			return nil
		},
	}
}
