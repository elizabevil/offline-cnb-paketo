package materials

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// ensureArchive 确保归档存在且通过 sha256 + size 校验。
// 返回需要回填的 sha256/size（空表示已固定）；写回 lock 由 Sync 统一完成。
func ensureArchive(ctx context.Context, a *lock.Asset, dest string, opts Options) (string, int64, error) {
	locked, _ := lockSource(a, opts.Platform, opts)
	want, wantSize := locked.SHA256, locked.Size
	writable := !opts.Check && !opts.Offline && opts.LockPath != ""

	if want == "" {
		if !writable {
			return "", 0, fmt.Errorf("%s: sha256 not pinned", a.Ref())
		}
		// 允许手工预置：归档已在目标位置时直接采用
		if _, err := os.Stat(dest); err == nil {
			sha, size, err := fileDigest(dest)
			if err != nil {
				return "", 0, err
			}
			return sha, size, nil
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", 0, err
		}
		srcErrs := sourceErrors{}
		for _, url := range sources(a, opts.Platform) {
			if err := opts.Fetch.download(ctx, url, dest); err != nil {
				srcErrs.add(url, err)
				continue
			}
			sha, size, err := fileDigest(dest)
			if err != nil {
				return "", 0, err
			}
			return sha, size, nil
		}
		return "", 0, fmt.Errorf("%s: all sources failed%s", a.Ref(), srcErrs)
	}

	if ok, err := verifyArchive(dest, want, wantSize); err == nil && ok {
		return "", 0, nil
	}
	// 多架构共用同一归档：优先从其它平台的归档复制，避免重复下载
	if a.MultiArch && reuseMultiArch(dest, a, want, wantSize, opts) {
		return "", 0, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", 0, err
	}
	srcErrs := sourceErrors{}
	for _, url := range sources(a, opts.Platform) {
		if err := opts.Fetch.download(ctx, url, dest); err != nil {
			srcErrs.add(url, err)
			continue
		}
		ok, err := verifyArchive(dest, want, wantSize)
		if err != nil {
			return "", 0, err
		}
		if ok {
			return "", 0, nil
		}
	}
	return "", 0, fmt.Errorf("%s: all sources failed%s", a.Ref(), srcErrs)
}

// fileDigest 返回文件 sha256 与大小。
func fileDigest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

// sources 返回候选下载地址（环境变量镜像优先）。
func sources(a *lock.Asset, platform lock.Platform) []string {
	base, _ := a.URL(platform)
	if base == "" {
		return nil
	}
	out := []string{}
	if m := os.Getenv("MATERIALS_MIRROR"); m != "" {
		out = append(out, m+base)
	}
	out = append(out, base)
	// lock 声明的回退源：仅在传输/HTTP 错误时尝试（sha 不匹配不会回退）
	out = append(out, a.Mirrors(platform)...)
	return out
}

// verifyArchive 校验哈希与大小。
func verifyArchive(path, want string, wantSize int64) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return false, nil
	}
	if wantSize > 0 && st.Size() != wantSize {
		return false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == want, nil
}

// reuseMultiArch 从其它平台目录复制同一归档（multi_arch 物料）。
func reuseMultiArch(dest string, a *lock.Asset, want string, wantSize int64, opts Options) bool {
	urls := sources(a, opts.Platform)
	if len(urls) == 0 {
		return false
	}
	pattern := filepath.Join(opts.BundleDir, "*", "archives", a.ID, a.Version, filepath.Base(urls[0]))
	matches, _ := filepath.Glob(pattern)
	for _, src := range matches {
		if src == dest {
			continue
		}
		if ok, _ := verifyArchive(src, want, wantSize); !ok {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return false
		}
		if err := fsutil.CopyExec(src, dest); err != nil {
			return false
		}
		return true
	}
	return false
}
