package cli

import (
	"github.com/spf13/cobra"
	"learn-cnb-paketo/tools/cnb/internal/materials"
)

func newPinCmd() *cobra.Command {
	return &cobra.Command{
		Use: "pin [ids...]", Short: "compute sha256 from local archives (prints only)",
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
