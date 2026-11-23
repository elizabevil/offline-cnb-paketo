package cli

import (
	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/materials"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

func newVerifyCmd() *cobra.Command {
	var quick bool
	cmd := &cobra.Command{
		Use: "verify [ids...]", Short: "verify archive hashes, unpack markers, and expected files",
		RunE: func(_ *cobra.Command, ids []string) error {
			ctx, err := workspace.Open(workspace.Options{
				Repo: opts.repo, LockPath: opts.lockPath, Bundle: opts.bundle, Platform: lock.Platform(opts.platform),
			})
			if err != nil {
				return err
			}
			if ctx.LockFile != nil && ctx.LockFile.Manifest.SHA256 != "" && ctx.LockFile.Manifest.SHA256 != ctx.ManifestHash {
				return &exitError{code: ExitFailure, msg: "assets.toml changed; run cnb materials lock first"}
			}
			rep, err := materials.Check(ctx.Lock, ctx.Bundle, ctx.Platform, ids, ctx.LockFile, quick)
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			if !printReport(rep, opts.strict, opts.quiet) {
				return &exitError{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&quick, "quick", false, "skip archive hashing; check size and unpack markers only")
	return cmd
}
