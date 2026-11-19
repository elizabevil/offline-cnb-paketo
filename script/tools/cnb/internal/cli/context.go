// Package cli implements the cnb command line: cobra commands, flags, and output.
package cli

import (
	"errors"
	"fmt"
	"os"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// opts holds global command line options.
var opts options

// Version metadata comes from internal/version buildinfo.

type options struct {
	lockPath  string
	bundle    string
	platform  string
	strict    bool
	quiet     bool
	repo      string
	configDir string
}

// exitError carries an exit code; an empty msg means the caller already printed it.
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

func open() (*lock.File, lock.Platform, string, error) {
	ctx, err := workspace.Open(workspace.Options{
		Repo: opts.repo, LockPath: opts.lockPath, Bundle: opts.bundle, Platform: lock.Platform(opts.platform),
	})
	if err != nil {
		return nil, "", "", err
	}
	return ctx.Lock, ctx.Platform, ctx.Bundle, nil
}

// resolveRepo resolves the project root through the workspace package, so the
// CLI and the library agree on every fallback (including the current directory).
func resolveRepo() (string, error) {
	root, err := workspace.Resolve(opts.repo)
	if err != nil {
		return "", errs.Wrap("locate repo root", err)
	}
	return root, nil
}

// workspaceLockPath is a thin wrapper for callers that only need the path.

// mustProfilePath returns project/<name>/pipeline.toml relative to the project root.

// Execute runs the CLI and maps exitError to a process exit code.
func Execute() {
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
