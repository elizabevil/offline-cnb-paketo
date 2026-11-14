package main

import (
	"github.com/spf13/cobra"
	"learn-cnb-paketo/tools/cnb/internal/materials"
)

func newPinCmd() *cobra.Command {
	return &cobra.Command{
		Use: "pin [ids...]", Short: "由本地归档反推 sha256（只打印，不写回）",
		RunE: func(_ *cobra.Command, ids []string) error {
			l, platform, bundleDir, err := open()
			if err != nil {
				return err
			}
			suggests, err := materials.Suggestions(l, bundleDir, platform, ids)
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			if !printSuggestions(suggests) {
				return &exitError{code: 1}
			}
			return nil
		},
	}
}
