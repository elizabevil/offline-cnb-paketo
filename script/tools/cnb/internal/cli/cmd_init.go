package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/scaffold"
)

// initOptions holds `cnb init` flags.
type initOptions struct {
	dir       string
	platform  string
	platforms string
	baseImage string
	container string
	only      []string
	skip      []string
	force     bool
	list      bool
}

// newInitCmd scaffolds a new project from the template registry.
func newInitCmd() *cobra.Command {
	var opts initOptions
	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "scaffold a project: pipeline config, assets, Dockerfiles",
		Long: "Render the project templates (config, assets, Dockerfiles) into <dir>.\n" +
			"Existing files are kept unless --force is given; --only/--skip select templates.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.list {
				return listTemplates(cmd)
			}
			name := args[0]
			o := scaffold.Options{
				Name:      name,
				Dir:       opts.dir,
				Platform:  opts.platform,
				Platforms: splitList(opts.platforms),
				BaseImage: opts.baseImage,
				Container: opts.container,
			}
			if o.Dir == "" {
				o.Dir = filepath.Join("project", name)
			}
			files, err := scaffold.Render(o, opts.only, opts.skip)
			if err != nil {
				return failExecution(err)
			}
			for _, f := range files {
				if fsutil.Exists(f.Path) && !opts.force {
					fmt.Fprintf(cmd.OutOrStdout(), "  · %-18s %s (exists, skipped)\n", f.ID, f.Path)
					continue
				}
				if err := writeFile(f.Path, f.Content); err != nil {
					return failExecution(err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %-18s %s\n", f.ID, f.Path)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.dir, "dir", "", "project directory (default project/<name>)")
	cmd.Flags().StringVar(&opts.platform, "platform", "", "default platform for the pipeline (default linux-amd64)")
	cmd.Flags().StringVar(&opts.platforms, "platforms", "", "platforms for assets.toml (default linux-amd64,linux-arm64)")
	cmd.Flags().StringVar(&opts.baseImage, "base-image", "", "base image for [spec.project.base]")
	cmd.Flags().StringVar(&opts.container, "container", os.Getenv("CNB_CONTAINER"), "container CLI for stages: podman | docker")
	cmd.Flags().StringSliceVar(&opts.only, "only", nil, "render only these template ids")
	cmd.Flags().StringSliceVar(&opts.skip, "skip", nil, "skip these template ids")
	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite existing files")
	cmd.Flags().BoolVar(&opts.list, "list", false, "list available templates")
	return cmd
}

// listTemplates prints the template registry.
func listTemplates(cmd *cobra.Command) error {
	for _, t := range scaffold.Templates() {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-18s %s\n", t.ID, t.Description)
	}
	return nil
}

// splitList parses a comma separated flag value.
func splitList(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for item := range strings.SplitSeq(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// writeFile creates parent directories and writes content.
func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), fsutil.DirMode); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), fsutil.FileMode)
}
