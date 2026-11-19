package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/scaffold"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// initOptions holds `cnb init` flags.
type initOptions struct {
	dir   string
	from  string
	only  []string
	skip  []string
	force bool
}

// newInitCmd scaffolds a new project from a reference project.
func newInitCmd() *cobra.Command {
	var opts initOptions
	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "scaffold a project from a reference project",
		Long: "Copy a working project (config, materials, Dockerfiles, justfile) and\n" +
			"rewrite the project name and directory. The reference project is the\n" +
			"source of truth, so generated projects cannot drift from it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			from := opts.from
			if from == "" {
				from = filepath.Join("project", "offline-paketo")
			}
			dir := opts.dir
			if dir == "" {
				dir = filepath.Join("project", name)
			}
			files, err := scaffold.Render(scaffold.Options{
				Name: name, Dir: dir, From: from, Only: opts.only, Skip: opts.skip,
				Assets: assetsFor(),
			})
			if err != nil {
				return failExecution(err)
			}
			hasPipeline := false
			for _, f := range files {
				if f.Path == "pipeline.toml" {
					hasPipeline = true
				}
			}
			if !hasPipeline {
				fmt.Fprintf(cmd.OutOrStdout(), "  ! pipeline.toml 未被选中：加 --only pipeline.toml 或去掉 --only\n")
			}
			for _, f := range files {
				dest := filepath.Join(dir, f.Path)
				if fsutil.Exists(dest) && !opts.force {
					fmt.Fprintf(cmd.OutOrStdout(), "  · %-28s (exists, skipped)\n", dest)
					continue
				}
				if err := writeFile(dest, f.Content); err != nil {
					return failExecution(err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", dest)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.dir, "dir", "", "project directory (default project/<name>)")
	cmd.Flags().StringVar(&opts.from, "from", "", "reference project (default project/offline-paketo)")
	cmd.Flags().StringSliceVar(&opts.only, "only", nil, "copy only these paths (relative)")
	cmd.Flags().StringSliceVar(&opts.skip, "skip", nil, "skip these paths (relative)")
	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite existing files")
	return cmd
}

// assetsFor returns the manifest next to the reference repo root, if any.
func assetsFor() string {
	root, err := workspace.Resolve("")
	if err != nil {
		return ""
	}
	for _, candidate := range []string{
		filepath.Join(root, "project", "assets.toml"),
		filepath.Join(root, "assets.toml"),
	} {
		if fsutil.Exists(candidate) {
			return candidate
		}
	}
	return ""
}

// writeFile creates parent directories and writes content.
func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), fsutil.DirMode); err != nil {
		return err
	}
	return os.WriteFile(path, content, fsutil.FileMode)
}
