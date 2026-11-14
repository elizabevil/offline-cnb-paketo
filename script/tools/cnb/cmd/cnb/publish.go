package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// newPublishCmd — 把本地构建产物发布成物料（复制 + 回填 sha256）
func newPublishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "publish <id[@version]>",
		Short: "把 [asset.local] 的构建产物发布到 bundle 并回填哈希",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveLockPath(opts.lockPath)
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
				platform = hostPlatform()
			}
			dst := filepath.Join(root, bundleDir, platform, "binaries", a.Dir(), name)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := copyFile(src, dst); err != nil {
				return err
			}
			sum, err := sha256File(dst)
			if err != nil {
				return err
			}
			changed, err := lock.SetLocalSHA(path, a.Ref(), sum)
			if err != nil {
				return &exitError{code: 1, msg: err.Error()}
			}
			fmt.Printf("  ✓ %s → %s\n", a.Ref(), dst)
			if changed {
				fmt.Printf("  + 已固定 [asset.local].sha256 = %s\n", sum[:12])
			}
			return nil
		},
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, 0o755)
}

func sha256File(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
