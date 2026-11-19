package cli

import (
	"fmt"
	"learn-cnb-paketo/tools/cnb/internal/checks"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	specprofile "learn-cnb-paketo/tools/cnb/internal/spec/profile"
	"path/filepath"
	"strings"

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
				prof, _, perr := loadProfileFile(args[0])
				if perr == nil {
					for _, f := range checks.CheckDockerfiles(dockerfileTargets(prof, root, p.Bundle), p.Bundle, filepath.Join(root, "buildpacks")) {
						findings = append(findings, lintFinding{Stage: f.Stage, Message: f.Describe(), IsError: true})
					}
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

// dockerfileTargets resolves spec.checks.dockerfiles against the repo root.
func dockerfileTargets(f *specprofile.File, repo, bundle string) []checks.DockerfileTarget {
	projectDir := filepath.Join(repo, f.Spec.Project.Dir)
	expand := func(p string) string {
		p = strings.ReplaceAll(p, "${repo}", repo)
		return strings.ReplaceAll(p, "${bundle}", bundle)
	}
	resolve := func(base, p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(base, p)
	}
	out := make([]checks.DockerfileTarget, 0, len(f.Spec.Checks.Dockerfiles))
	for _, dc := range f.Spec.Checks.Dockerfiles {
		contextDir := resolve(projectDir, expand(dc.Context))
		if dc.Context == "" {
			contextDir = projectDir
		}
		contexts := make(map[string]string, len(dc.Contexts))
		for name, value := range dc.Contexts {
			contexts[name] = resolve(repo, expand(value))
		}
		out = append(out, checks.DockerfileTarget{
			Name:       dc.Path,
			Dockerfile: filepath.Join(projectDir, dc.Path),
			Context:    contextDir,
			Contexts:   contexts,
		})
	}
	return out
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
