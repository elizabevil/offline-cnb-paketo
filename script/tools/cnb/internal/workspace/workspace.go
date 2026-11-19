// Package workspace 解析仓库根、物料清单与 bundle 目录等运行期路径。
package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
)

// 环境变量与默认值。
const (
	EnvRepoRoot    = "CNB_REPO_ROOT"
	EnvLockFile    = "LOCK_FILE"
	EnvRootMarkers = "CNB_ROOT_MARKERS"

	DefaultManifestName = lock.DefaultManifestName
	DefaultLockName     = lock.DefaultLockName
	DefaultBundle       = lock.DefaultBundle

	PlatformAMD64 = lock.PlatformAMD64
	PlatformARM64 = lock.PlatformARM64
)

// Options 是调用方显式指定的覆盖项。
type Options struct {
	Repo     string
	LockPath string
	Bundle   string
	Platform lock.Platform
}

// Context 是一次命令执行所需的路径上下文。
type Context struct {
	Root string
	// LockPath 是物料清单（assets.toml）。
	LockPath string
	Lock     *lock.File
	// LockFilePath 是生成的 assets.lock.toml。
	LockFilePath string
	// LockFile 是已解析的校验信息（可能为空锁）。
	LockFile *manifest.LockFile
	// ManifestHash 是当前清单的指纹，用于 lock 漂移检测。
	ManifestHash string
	Bundle       string
	Platform     lock.Platform
}

// Open 解析路径并加载物料清单。
func Open(o Options) (Context, error) {
	root, err := resolveRepo(o.Repo)
	if err != nil {
		// An explicit lock path makes the project root optional: fall back to
		// the directory holding the manifest so any layout works.
		if o.LockPath == "" {
			return Context{}, err
		}
		abs, absErr := filepath.Abs(o.LockPath)
		if absErr != nil {
			return Context{}, err
		}
		root = filepath.Dir(abs)
	}
	lockPath := resolveManifest(o.LockPath, root)
	l, err := lock.Load(lockPath)
	if err != nil {
		return Context{}, err
	}
	platform := o.Platform
	if platform == "" {
		platform = HostPlatform()
	}
	bundleDir := o.Bundle
	if bundleDir == "" {
		bundleDir = resolveBundle(filepath.Dir(lockPath), l)
	}
	lockFilePath := filepath.Join(filepath.Dir(lockPath), DefaultLockName)
	lf, err := manifest.LoadLockFile(lockFilePath)
	if err != nil {
		return Context{}, err
	}
	manifestHash, err := manifest.Hash(l)
	if err != nil {
		return Context{}, err
	}
	return Context{
		Root: root, LockPath: lockPath, Lock: l,
		LockFilePath: lockFilePath, LockFile: lf, ManifestHash: manifestHash,
		Bundle: bundleDir, Platform: platform,
	}, nil
}

// Resolve returns the project root: explicit path, CNB_REPO_ROOT, a current
// directory holding a manifest, then marker-file discovery.
func Resolve(explicit string) (string, error) { return resolveRepo(explicit) }

// resolveManifest returns the manifest path: flag > env > ./assets.toml.
func resolveManifest(explicit, root string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv(EnvLockFile); env != "" {
		return env
	}
	if wd, err := os.Getwd(); err == nil {
		if cand := filepath.Join(wd, DefaultManifestName); fsutil.Exists(cand) {
			return cand
		}
	}
	return filepath.Join(root, DefaultManifestName)
}

// resolveRepo 解析仓库根：显式参数 > 环境变量 > 标记文件定位。// resolveRepo 解析仓库根：显式参数 > 环境变量 > 标记文件定位。
func resolveRepo(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	if env := os.Getenv(EnvRepoRoot); env != "" {
		return filepath.Abs(env)
	}
	// A directory holding a manifest is a project root: the tool may run
	// outside the repository against an arbitrary tree.
	if wd, err := os.Getwd(); err == nil {
		if fsutil.Exists(filepath.Join(wd, DefaultManifestName)) {
			return wd, nil
		}
	}
	if root, err := FindFromExecutable(); err == nil {
		return root, nil
	}
	root, err := FindFromWorkingDir()
	if err != nil {
		return "", errs.Wrap("locate repo root", err)
	}
	return root, nil
}

// HostPlatform 返回当前主机的物料平台名。
func HostPlatform() lock.Platform {
	if runtime.GOARCH == "arm64" {
		return lock.Platform(PlatformARM64)
	}
	return lock.Platform(PlatformAMD64)
}

// ProfilePath returns <root>/project/<name>/pipeline.toml.
//
// Each project keeps its pipeline config next to its files, so a project
// directory is self-contained and can be copied as a whole.
func ProfilePath(root, configDir, name string) string {
	if configDir == "" {
		configDir = "project"
	}
	return filepath.Join(root, configDir, name, "pipeline.toml")
}

// UserProfilePath 把 name 或路径统一成配置文件路径。
func UserProfilePath(root, configDir, name string) string {
	if strings.HasSuffix(name, ".toml") || strings.ContainsRune(name, '/') {
		if filepath.IsAbs(name) {
			return name
		}
		return filepath.Join(root, name)
	}
	return ProfilePath(root, configDir, name)
}

// resolveBundle resolves [bundle].path against the manifest directory.
func resolveBundle(lockDir string, l *lock.File) string {
	root := l.Bundle.Path
	if root == "" {
		root = DefaultBundle
	}
	if filepath.IsAbs(root) {
		return root
	}
	return filepath.Join(lockDir, root)
}
