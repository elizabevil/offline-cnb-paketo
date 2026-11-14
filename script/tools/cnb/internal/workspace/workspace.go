// Package workspace 解析仓库根、物料清单与 bundle 目录等运行期路径。
package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/strutil"
)

// 环境变量与默认值。
const (
	EnvRepoRoot = "CNB_REPO_ROOT"
	EnvLockFile = "LOCK_FILE"

	DefaultLockName = lock.DefaultLockName
	DefaultBundle   = lock.DefaultBundle

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
	Root     string
	LockPath string
	Lock     *lock.File
	Bundle   string
	Platform lock.Platform
}

// Open 解析路径并加载物料清单。
func Open(o Options) (Context, error) {
	root, err := resolveRepo(o.Repo)
	if err != nil {
		return Context{}, err
	}
	lockPath := o.LockPath
	if lockPath == "" {
		lockPath = strutil.FirstNonEmpty(os.Getenv(EnvLockFile), filepath.Join(root, DefaultLockName))
	}
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
		bundleDir = filepath.Join(filepath.Dir(lockPath), DefaultBundle)
	}
	return Context{Root: root, LockPath: lockPath, Lock: l, Bundle: bundleDir, Platform: platform}, nil
}

// resolveRepo 解析仓库根：显式参数 > 环境变量 > 标记文件定位。
func resolveRepo(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	if env := os.Getenv(EnvRepoRoot); env != "" {
		return filepath.Abs(env)
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

// ProfilePath 返回 config/projects/<name>.toml 的绝对路径。
func ProfilePath(root, configDir, name string) string {
	if configDir == "" {
		configDir = "config"
	}
	return filepath.Join(root, configDir, "projects", name+".toml")
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
