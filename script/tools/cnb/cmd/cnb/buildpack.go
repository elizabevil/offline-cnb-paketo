package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/bpcheck"
	"learn-cnb-paketo/tools/cnb/internal/render"
)

// newBuildpackCmd 是 buildpack 维护入口。
func newBuildpackCmd() *cobra.Command {
	parent := &cobra.Command{Use: "buildpack", Short: "buildpack maintenance"}
	parent.AddCommand(&cobra.Command{
		Use:   "lint [dir…]",
		Short: "check buildpack layout against official conventions",
		RunE: func(cmd *cobra.Command, dirs []string) error {
			if len(dirs) == 0 {
				root, err := resolveRepo()
				if err != nil {
					return &exitError{code: ExitFailure, msg: err.Error()}
				}
				dirs, _ = filepath.Glob(filepath.Join(root, "buildpacks", "*", "*"))
			}
			out := cmd.OutOrStdout()
			var errs, warns int
			for _, dir := range dirs {
				rep := bpcheck.Check(dir)
				for _, f := range rep.Findings {
					mark := render.Warn("")
					if f.IsError {
						mark = render.Failure("")
					}
					fmt.Fprintf(out, "  %s %s: %s\n", mark, f.Name, f.Message)
				}
				errs += rep.Errors()
				warns += rep.Warnings()
			}
			fmt.Fprintf(out, "\n  %d error, %d warning\n", errs, warns)
			if errs > 0 {
				return &exitError{code: ExitFailure}
			}
			return nil
		},
	})
	return parent
}
