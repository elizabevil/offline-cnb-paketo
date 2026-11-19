package cli

import (
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/plan"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// newVariablesCmd prints every supported placeholder for a profile.
func newVariablesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "variables <profile>",
		Short: "list placeholders available in stage run commands",
		Long: "Print each supported ${...} placeholder with its resolved value and source.\n" +
			"Unknown placeholders fail at plan time with the same table in mind.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, _, err := loadProfileFile(args[0])
			if err != nil {
				return err
			}
			if opts.platform != "" {
				f.Spec.Project.Platform = opts.platform
			}
			projectRoot, manifest := projectRoots(f, args[0])
			ws, err := workspace.Open(workspace.Options{
				Repo: opts.repo, LockPath: manifest, Platform: lock.Platform(opts.platform),
			})
			if err != nil {
				return failExecution(err)
			}
			rows, err := plan.Placeholders(f, ws.Lock, plan.Options{
				Project: projectRoot, Materials: filepath.Dir(manifest), Repo: ws.Root, Bundle: ws.Bundle,
			})
			if err != nil {
				return failExecution(err)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "PLACEHOLDER\tVALUE\tSOURCE")
			for _, r := range rows {
				fmt.Fprintf(w, "${%s}\t%s\t%s\n", r.Name, r.Value, r.Source)
			}
			return w.Flush()
		},
	}
}
