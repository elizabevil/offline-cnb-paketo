package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// newPublishCmd — 把本地构建产物发布成物料（复制 + 回填 sha256）
func newPublishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "publish <id[@version]>",
		Short: "把 [asset.local] 的构建产物发布到 bundle 并回填哈希",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := workspaceLockPath(opts.lockPath)
			if err != nil {
				return err
			}
			l, err := lock.Load(path)
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			a, err := lock.Find(l, args[0])
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			if !a.IsLocal() {
				return &exitError{code: 2, msg: a.Ref() + " 不是本地物料（缺少 [asset.local]）"}
			}
			root, err := resolveRepo()
			if err != nil {
				return err
			}
			src := filepath.Join(root, a.Local.Source)
			if _, err := os.Stat(src); err != nil {
				return &exitError{code: 1, msg: "找不到构建产物: " + src}
			}
			name := a.Unpack.Expect[0]
			bundleDir := opts.bundle
			if bundleDir == "" {
				bundleDir = "bundle"
			}
			platform := opts.platform
			if platform == "" {
				platform = string(workspace.HostPlatform())
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
			changed, err := lock.SetLocalSHA(path, a.Ref(), sum)
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			logr.Info("published", "ref", a.Ref(), "dest", dst)
			if changed {
				logr.Info("pinned local sha256", "ref", a.Ref(), "sha256", sum)
			}
			return nil
		},
	}
}
