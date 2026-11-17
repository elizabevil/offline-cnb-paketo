package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/checks"
)

// newLintCmd 检查 buildpack 结构是否符合官方约定。
func newLintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint [dir…]",
		Short: "check buildpack layout against official conventions",
		RunE: func(_ *cobra.Command, dirs []string) error {
			if len(dirs) == 0 {
				root, err := resolveRepo()
				if err != nil {
					return &exitError{code: ExitFailure, msg: err.Error()}
				}
				dirs, _ = filepath.Glob(filepath.Join(root, "buildpacks", "*", "*"))
			}
			var errs, warns int
			for _, dir := range dirs {
				rep := checks.CheckBuildpack(dir)
				for _, f := range rep.Findings {
					if f.IsError {
						logr.Error("buildpack lint", "buildpack", f.Name, "problem", f.Message)
						continue
					}
					logr.Warn("buildpack lint", "buildpack", f.Name, "problem", f.Message)
				}
				errs += rep.Errors()
				warns += rep.Warnings()
			}
			logr.Info("buildpack lint summary", "errors", errs, "warnings", warns)
			if errs > 0 {
				return &exitError{code: ExitFailure}
			}
			return nil
		},
	}
	return cmd
}
