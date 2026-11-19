package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/checks"
)

// newBuildpackCmd groups buildpack structure checks and builder metadata.
func newLintCmd() *cobra.Command {
	var rulesPath string
	cmd := &cobra.Command{
		Use:   "lint [dir…]",
		Short: "check buildpack layout against official conventions",
		RunE: func(_ *cobra.Command, dirs []string) error {
			if len(dirs) == 0 {
				root, err := resolveRepo()
				if err != nil {
					return failExecution(err)
				}
				dirs, _ = filepath.Glob(filepath.Join(root, "buildpacks", "*", "*"))
			}
			rules, err := checks.LoadRuleset(rulesPath)
			if err != nil {
				return &exitError{code: ExitFailure, msg: "load ruleset: " + err.Error()}
			}
			var errs, warns int
			for _, dir := range dirs {
				rep := checks.CheckBuildpackWith(dir, rules)
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
	cmd.Flags().StringVar(&rulesPath, "rules", "", "external ruleset TOML (defaults to the embedded ruleset)")
	return cmd
}
