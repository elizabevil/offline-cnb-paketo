package main

import (
	"github.com/spf13/cobra"
	"learn-cnb-paketo/tools/cnb/internal/repack"
)

func newRepackCmd() *cobra.Command {
	return &cobra.Command{
		Use: "repack <SmartPackage 目录 | *.tar.gz>", Short: "把 docker-save 风格的 web.tgz 规范化为 OCI layout",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := repack.Run(args[0]); err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			return nil
		},
	}
}
