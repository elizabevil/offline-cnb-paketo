package materials

import (
	"context"
	"fmt"
	"path/filepath"

	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
)

// Generate 根据清单生成/刷新 assets.lock.toml。
// 已存在且校验通过的归档会被直接采用，不重复下载。
func Generate(m *manifest.File, lf *manifest.LockFile, ids []string, opts Options) error {
	if lf == nil || opts.LockFilePath == "" {
		return fmt.Errorf("缺少 lock 文件路径")
	}
	assets, err := m.Select(ids)
	if err != nil {
		return err
	}
	for i := range assets {
		a := &assets[i]
		for _, p := range m.Meta.Platforms {
			platform := lock.Platform(p)
			src, err := lockedSourceFor(a, platform, opts)
			if err != nil {
				return err
			}
			lf.Set(a, p, src)
		}
	}
	lf.Manifest.SHA256 = opts.ManifestHash
	opts.Logger.Info("lock written", "path", opts.LockFilePath)
	return manifest.SaveLockFile(opts.LockFilePath, lf)
}

// lockedSourceFor 计算某平台在 lock 中应有的条目。
func lockedSourceFor(a *manifest.Asset, platform lock.Platform, opts Options) (manifest.LockedSource, error) {
	binDir := binariesDir(opts.BundleDir, platform)
	if a.IsLocal(platform) {
		rel := a.ExpectedPaths(platform)[0]
		sha, _, err := fileDigest(filepath.Join(binDir, rel))
		if err != nil {
			return manifest.LockedSource{}, fmt.Errorf("%s[%s]: 计算本地产物哈希失败: %w", a.Ref(), platform, err)
		}
		return manifest.LockedSource{SHA256: sha}, nil
	}

	dest := archivePath(opts.BundleDir, platform, a)
	sha, size, err := ensureArchive(context.Background(), a, dest, opts)
	if err != nil {
		return manifest.LockedSource{}, err
	}
	if sha == "" {
		// 归档已存在且校验通过，直接计算
		sha, size, err = fileDigest(dest)
		if err != nil {
			return manifest.LockedSource{}, err
		}
	}
	url, _ := a.URL(platform)
	return manifest.LockedSource{URL: url, SHA256: sha, Size: size}, nil
}
