package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/exec"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/plan"
	"learn-cnb-paketo/tools/cnb/internal/render"
	specprofile "learn-cnb-paketo/tools/cnb/internal/spec/profile"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// loadPipeline parses a pipeline config and builds the execution plan.
func loadPipeline(path string) (*specprofile.File, *plan.Plan, error) {
	f, warns, err := loadProfileFile(path)
	if err != nil {
		return nil, nil, err
	}
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "  ! %s\n", w.String())
	}
	// --platform overrides the profile platform (e.g. cross-checking arm64).
	if opts.platform != "" {
		f.Spec.Project.Platform = opts.platform
	}
	if errsOut := f.Validate(); len(errsOut) > 0 {
		for _, e := range errsOut {
			fmt.Fprintf(os.Stderr, "  ✗ %v\n", e)
		}
		return nil, nil, &exitError{code: ExitFailure}
	}
	root := rootForProfile(f, path)
	ctx, err := workspace.Open(workspace.Options{
		Repo:     root,
		LockPath: lockForProfile(f, root, path),
		Platform: lock.Platform(opts.platform),
	})
	if err != nil {
		return nil, nil, err
	}
	p, _, err := plan.Build(f, ctx.Lock, plan.Options{Repo: ctx.Root, Bundle: ctx.Bundle})
	if err != nil {
		return nil, nil, failExecution(err)
	}
	return f, p, nil
}

// rootForProfile derives the project root from the profile location.
//
// The profile lives inside the project (e.g. project/demo/pipeline.toml), so the
// root is the parent of spec.project.directory. An explicit --repo wins.
func rootForProfile(f *specprofile.File, profilePath string) string {
	if opts.repo != "" {
		return opts.repo
	}
	absProfile, err := filepath.Abs(profilePath)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(absProfile)
	projDir := filepath.Clean(f.Spec.Project.Directory)
	suffix := string(filepath.Separator) + projDir
	if root, ok := strings.CutSuffix(dir, suffix); ok {
		return root
	}
	return ""
}

// lockForProfile finds the manifest next to the project or the profile.
//
// A scaffolded project keeps assets.toml inside the project directory, so the
// repo-root default does not apply.
func lockForProfile(f *specprofile.File, root, profilePath string) string {
	if opts.lockPath != "" {
		return opts.lockPath
	}
	candidates := []string{
		filepath.Join(root, f.Spec.Project.Directory, "assets.toml"),
		filepath.Join(filepath.Dir(mustAbs(profilePath)), "assets.toml"),
	}
	for _, c := range candidates {
		if fsutil.Exists(c) {
			return c
		}
	}
	return ""
}

// mustAbs makes a path absolute, returning it unchanged on error.
func mustAbs(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// newPlanCmd prints the execution plan (final argv per stage).
func newPlanCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "plan <profile>",
		Short: "print the execution plan (stages + final argv)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, p, err := loadPipeline(args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if format == "json" {
				return writePlanJSON(out, f.Metadata.Name, p)
			}
			fmt.Fprintf(out, "project: %s\nplatform: %s\nbundle: %s\nstages:\n", f.Metadata.Name, p.Platform, p.Bundle)
			for i, st := range p.Stages {
				fmt.Fprintf(out, "  %d. %s\n", i+1, st.Name)
				if st.Description != "" {
					fmt.Fprintf(out, "     # %s\n", st.Description)
				}
				fmt.Fprintf(out, "     cd %s\n     %v\n", st.Workdir, st.Argv)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text | json")
	return cmd
}

// writePlanJSON renders the plan as valid JSON.
func writePlanJSON(out io.Writer, project string, p *plan.Plan) error {
	type stageJSON struct {
		Name        string   `json:"name"`
		Description string   `json:"description,omitempty"`
		Workdir     string   `json:"workdir"`
		Argv        []string `json:"argv"`
	}
	type planJSON struct {
		Project  string      `json:"project"`
		Platform string      `json:"platform"`
		Bundle   string      `json:"bundle"`
		Stages   []stageJSON `json:"stages"`
	}
	doc := planJSON{Project: project, Platform: p.Platform, Bundle: p.Bundle}
	for _, st := range p.Stages {
		doc.Stages = append(doc.Stages, stageJSON{Name: st.Name, Description: st.Description, Workdir: st.Workdir, Argv: st.Argv})
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// newRunCmd executes the plan stage by stage.
func newRunCmd() *cobra.Command {
	var (
		dryRun   bool
		from     string
		until    string
		logLevel string
	)
	cmd := &cobra.Command{
		Use:   "run <profile>",
		Short: "run the pipeline in stage order",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			_, p, err := loadPipeline(args[0])
			if err != nil {
				return err
			}
			logger := render.NewLogger(os.Stderr, logLevel)
			if dryRun {
				_, err := exec.Run(context.Background(), p, exec.Options{DryRun: true}, logger)
				return err
			}
			if _, err := exec.Run(context.Background(), p, exec.Options{From: from, Until: until}, logger); err != nil {
				return failExecution(err)
			}
			logger.Info("pipeline done", "project", p.Project)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print commands without executing")
	cmd.Flags().StringVar(&from, "from", "", "start at this stage, skipping earlier ones")
	cmd.Flags().StringVar(&until, "until", "", "stop after this stage (inclusive)")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "log level: debug|info|warn|error")
	return cmd
}
