package main

import (
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/bundle"
)

// printReport 输出校验报告并返回是否通过。
func printReport(rep bundle.Report, strict, quiet bool) bool {
	logr.Info("verify assets", "platform", rep.Platform)
	for _, r := range rep.Results {
		switch r.Status {
		case bundle.StatusOK:
			if !quiet {
				logr.Info("asset ok", "ref", r.Ref)
			}
		case bundle.StatusUnpinned:
			logr.Warn("asset unpinned", "ref", r.Ref, "problems", strings.Join(r.Problems, "; "))
		default:
			logr.Error("asset broken", "ref", r.Ref, "problems", strings.Join(r.Problems, "; "))
		}
	}
	if rep.OK(strict) {
		logr.Info("verify passed", "unpinned", rep.Unpinned())
		return true
	}
	if rep.Unpinned() > 0 && !strict {
		logr.Warn("assets without pinned sha256", "count", rep.Unpinned())
	}
	logr.Error("verify failed", "broken", rep.Failures())
	return false
}

// printSuggestions 输出 sha256 回填建议。
func printSuggestions(suggests []bundle.Suggestion) bool {
	if len(suggests) == 0 {
		logr.Warn("no archives to pin")
		return false
	}
	logr.Info("review snippets before writing assets.lock.toml")
	for _, s := range suggests {
		logr.Info("sha256 suggestion", "ref", s.Ref, "platform", s.Platform, "sha256", s.SHA256)
	}
	return true
}
