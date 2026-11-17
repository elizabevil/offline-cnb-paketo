// cnb — 基座小工具的命令行入口。
//
// CLI 层（cobra、flag、输出格式）只在这里；业务逻辑在 internal/：
//
//	internal/lock     物料清单解析与校验
//	internal/bundle   物料落盘状态校验 / 哈希固定
//	internal/repack   SmartPackage 规范化
//	internal/shell    shell 字面量转义
package main

import (
	"errors"
	"fmt"
	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
	"os"
	"path/filepath"
)

// opts 保存全局命令行选项。
var opts options

// 版本由 internal/version 的 buildinfo 提供

type options struct {
	lockPath  string
	bundle    string
	platform  string
	strict    bool
	quiet     bool
	repo      string
	configDir string
}

// exitError 携带特定退出码；msg 为空表示错误已由调用方打印。
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string {
	if e.msg == "" {
		return fmt.Sprintf("exit %d", e.code)
	}
	return e.msg
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		if ee, ok := errors.AsType[*exitError](err); ok {
			if ee.msg != "" {
				fmt.Fprintf(os.Stderr, "cnb: %s\n", ee.msg)
			}
			os.Exit(ee.code)
		}
		fmt.Fprintf(os.Stderr, "cnb: %v\n", err)
		os.Exit(1)
	}
}

func open() (*lock.File, lock.Platform, string, error) {
	ctx, err := workspace.Open(workspace.Options{
		Repo: opts.repo, LockPath: opts.lockPath, Bundle: opts.bundle, Platform: lock.Platform(opts.platform),
	})
	if err != nil {
		return nil, "", "", err
	}
	return ctx.Lock, ctx.Platform, ctx.Bundle, nil
}

// resolveRepo 解析仓库根：--repo > CNB_REPO_ROOT > 可执行文件位置推导。
func resolveRepo() (string, error) {
	if opts.repo != "" {
		return filepath.Abs(opts.repo)
	}
	if env := os.Getenv("CNB_REPO_ROOT"); env != "" {
		return filepath.Abs(env)
	}
	if root, err := workspace.FindFromExecutable(); err == nil {
		return root, nil
	}
	root, err := workspace.FindFromWorkingDir()
	if err != nil {
		return "", errs.Wrap("locate repo root", err)
	}
	return root, nil
}

// workspaceLockPath 是 workspace 解析的薄包装，供少数只取路径的调用点复用。
func workspaceLockPath(explicit string) (string, error) {
	ctx, err := workspace.Open(workspace.Options{Repo: opts.repo, LockPath: explicit})
	if err != nil {
		return "", err
	}
	return ctx.LockPath, nil
}

// mustProfilePath 返回 config/projects/<name>.toml（相对仓库根）。
func mustProfilePath(name string) string {
	root, _ := resolveRepo()
	return filepath.Join(root, opts.configDir, "projects", name+".toml")
}
