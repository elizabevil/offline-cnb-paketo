package cli

import (
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use: "validate", Short: "校验 lock 结构", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			l, _, _, err := open()
			if err != nil {
				return err
			}
			if l.Validate() > 0 {
				return &exitError{code: 1}
			}
			return nil
		},
	}
}
