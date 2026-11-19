package cli

import (
	"fmt"
	"learn-cnb-paketo/tools/cnb/internal/checks"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"path/filepath"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/plan"
)

// lintFinding is one lint result.
type lintFinding struct {
	Stage   string
	Message string
	IsError bool
}

// newLintCmd reports common pipeline mistakes (best effort).
func newPipelineLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint <profile>",
		Short: "check a pipeline for common mistakes (build args, scripts)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, err := loadPipeline(args[0])
			if err != nil {
				return err
			}
			var findings []lintFinding
			for _, st := range p.Stages {
				findings = append(findings, lintStage(st)...)
			}
			root, err := resolveRepo()
			if err == nil {
				targets := make([]checks.DockerfileTarget, 0, len(p.Dockerfiles))
				for _, t := range p.Dockerfiles {
					targets = append(targets, checks.DockerfileTarget{
						Name: t.Name, Dockerfile: t.Dockerfile, Context: t.Context, Contexts: t.Contexts,
					})
				}
				for _, f := range checks.CheckDockerfiles(targets, p.Bundle, filepath.Join(root, "buildpacks")) {
					findings = append(findings, lintFinding{Stage: f.Stage, Message: f.Describe(), IsError: true})
				}
			}
			errsN := 0
			for _, f := range findings {
				mark := "  ⚠"
				if f.IsError {
					mark = "  ✗"
					errsN++
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", mark, f.Stage, f.Message)
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  ✓ no problems found")
			}
			if errsN > 0 {
				return &exitError{code: ExitFailure}
			}
			return nil
		},
	}
}

// lintStage checks one stage.
func lintStage(st plan.Stage) []lintFinding {
	var out []lintFinding
	add := func(err bool, format string, a ...any) {
		out = append(out, lintFinding{Stage: st.Name, Message: fmt.Sprintf(format, a...), IsError: err})
	}
	if len(st.Argv) == 0 {
		add(true, "empty run command")
		return out
	}
	if checks.IsShellScript(st.Argv) && !existsRel(st.Workdir, st.Argv[1]) {
		add(true, "script not found: %s", st.Argv[1])
	}
	return out
}

// existsRel reports whether a path exists relative to workdir.
func existsRel(workdir, p string) bool {
	if filepath.IsAbs(p) {
		return fsutil.Exists(p)
	}
	return fsutil.Exists(filepath.Join(workdir, p))
}
