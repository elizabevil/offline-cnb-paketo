package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// newPublishCmd publishes a local build artifact as a material (copy + pin sha256).
func newPublishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "publish <id[@version]>",
		Short: "publish [asset.local] build output into the bundle and pin its hash",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx, err := workspace.Open(workspace.Options{
				Repo: opts.repo, LockPath: opts.lockPath, Bundle: opts.bundle, Platform: lock.Platform(opts.platform),
			})
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			l := ctx.Lock
			a, err := lock.Find(l, args[0])
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			platform := opts.platform
			if platform == "" {
				platform = string(workspace.HostPlatform())
			}
			if !a.IsLocal(lock.Platform(platform)) {
				return &exitError{code: 2, msg: a.Ref() + " is not a local material (missing [asset.local])"}
			}
			root, err := resolveRepo()
			if err != nil {
				return err
			}
			srcTpl := a.Platform[platform].Source
			if srcTpl == "" {
				return &exitError{code: 2, msg: a.Ref() + " has no source for platform " + platform}
			}
			src := filepath.Join(root, srcTpl)
			if _, err := os.Stat(src); err != nil {
				return &exitError{code: 1, msg: "build output not found: " + src}
			}
			name := a.Unpack.Expect[0]
			bundleDir := opts.bundle
			if bundleDir == "" {
				bundleDir = "bundle"
			}
			dst := filepath.Join(root, bundleDir, platform, "binaries", a.Dir(), name)
			if err := os.MkdirAll(filepath.Dir(dst), fsutil.DirMode); err != nil {
				return err
			}
			if err := fsutil.CopyExec(src, dst); err != nil {
				return err
			}
			sum, err := fsutil.SHA256(dst)
			if err != nil {
				return err
			}
			ctx.LockFile.Set(a, platform, manifest.LockedSource{SHA256: sum})
			ctx.LockFile.Manifest.SHA256 = ctx.ManifestHash
			if err := manifest.SaveLockFile(ctx.LockFilePath, ctx.LockFile); err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			changed := true
			logr.Info("published", "ref", a.Ref(), "dest", dst)
			if changed {
				logr.Info("pinned local sha256", "ref", a.Ref(), "sha256", sum)
			}
			return nil
		},
	}
}
