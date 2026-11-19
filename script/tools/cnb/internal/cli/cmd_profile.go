package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/render"
	platformspec "learn-cnb-paketo/tools/cnb/internal/spec/platform"
	specprofile "learn-cnb-paketo/tools/cnb/internal/spec/profile"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// newProfileCmd covers pipeline config: generate, validate, show.
func newProfileCmd() *cobra.Command {
	parent := &cobra.Command{Use: "profile", Short: "pipeline config: init / validate / show"}
	parent.AddCommand(newProfileInitCmd())
	parent.AddCommand(newProfileValidateCmd())
	var showOpts profileShowOptions
	showCmd := &cobra.Command{
		Use: "show <file>", Short: "print the config as structured data", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, _, err := loadProfileFile(args[0])
			if err != nil {
				return err
			}
			out, err := render.FormatStruct(f, showOpts.format)
			if err != nil {
				return failUsage(err)
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	showCmd.Flags().StringVar(&showOpts.format, "format", "yaml", "output format: yaml | json")
	parent.AddCommand(showCmd)

	return parent
}

// newProfileInitCmd prints a new project config template.
// profileInitOptions holds profile init flags.
type profileInitOptions struct {
	dir       string
	platform  string
	baseImage string
}

func newProfileInitCmd() *cobra.Command {
	var opts profileInitOptions
	initCmd := &cobra.Command{
		Use: "init <name>", Short: "generate a new project config template", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir := opts.dir
			if dir == "" {
				dir = name
			}
			platform := opts.platform
			if platform == "" {
				platform = platformspec.DefaultTargetPlatform
			}
			out := filepath.Join("config", "projects", name+".toml")
			if _, err := os.Stat(out); err == nil {
				return &exitError{code: ExitFailure, msg: "already exists: " + out}
			}
			if err := os.MkdirAll(filepath.Dir(out), fsutil.DirMode); err != nil {
				return err
			}
			content, err := specprofile.Template(name, dir, platform, opts.baseImage)
			if err != nil {
				return failExecution(err)
			}
			if err := os.WriteFile(out, []byte(content), fsutil.FileMode); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ wrote %s\n", out)
			return nil
		},
	}
	initCmd.Flags().StringVar(&opts.dir, "dir", "", "project directory relative to the repo root (default: name)")
	initCmd.Flags().StringVar(&opts.platform, "platform", "", "default platform (default: linux-amd64)")
	initCmd.Flags().StringVar(&opts.baseImage, "base-image", "", "base image written into [spec.project.base] (default debian:trixie-slim)")
	return initCmd
}

// profileShowOptions holds profile show flags.
type profileShowOptions struct {
	format string
}

// newProfileValidateCmd validates a pipeline config.
func newProfileValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use: "validate <file>", Short: "validate a pipeline config", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, warns, err := loadProfileFile(args[0])
			if err != nil {
				return err
			}
			failed := 0
			for _, w := range warns {
				fmt.Fprintf(cmd.OutOrStdout(), "  ! %s\n", w.String())
			}
			for _, e := range f.Validate() {
				failed++
				fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %v\n", e)
			}
			if failed > 0 {
				return &exitError{code: ExitFailure}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ valid pipeline: %s (%d materials, %d stages, platform %s)\n",
				f.Metadata.Name, len(f.Spec.Assets), len(f.Spec.Stages), f.Spec.Project.Platform)
			return nil
		},
	}
}

// assertWorkspace keeps the workspace import in use.
var _ = workspace.Open
