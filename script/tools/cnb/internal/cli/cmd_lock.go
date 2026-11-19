package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/materials"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// newLockCmd generates or refreshes assets.lock.toml.
func newLockCmd() *cobra.Command {
	var jobs int
	cmd := &cobra.Command{
		Use:   "lock [ids...]",
		Short: "generate or refresh assets.lock.toml (download and hash)",
		RunE: func(cmd *cobra.Command, ids []string) error {
			ctx, err := workspace.Open(workspace.Options{
				Repo: opts.repo, LockPath: opts.lockPath, Bundle: opts.bundle, Platform: lock.Platform(opts.platform),
			})
			if err != nil {
				return failExecution(err)
			}
			if err := materials.Generate(ctx.Lock, ctx.LockFile, ids, materials.Options{
				BundleDir:    ctx.Bundle,
				Platform:     ctx.Platform,
				LockFile:     ctx.LockFile,
				LockFilePath: ctx.LockFilePath,
				Manifest:     ctx.Lock,
				ManifestHash: ctx.ManifestHash,
				Jobs:         jobs,
				Logger:       logr,
			}); err != nil {
				return failExecution(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", ctx.LockFilePath)
			return nil
		},
	}
	cmd.Flags().IntVar(&jobs, "jobs", 4, "parallel downloads")
	return cmd
}
