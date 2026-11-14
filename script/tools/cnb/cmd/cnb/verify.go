package main

import (
	"fmt"
	"os"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/bundle"
)

// printReport 输出校验报告并返回是否通过。
func printReport(rep bundle.Report, strict, quiet bool) bool {
	fmt.Printf("\n━━━  verify assets (%s)\n", rep.Platform)
	for _, r := range rep.Results {
		switch r.Status {
		case bundle.StatusOK:
			if !quiet {
				fmt.Printf("  ✓  %s\n", r.Ref)
			}
		case bundle.StatusUnpinned:
			fmt.Printf("  !  %s: %s\n", r.Ref, strings.Join(r.Problems, "; "))
		default:
			fmt.Printf("  ✗  %s: %s\n", r.Ref, strings.Join(r.Problems, "; "))
		}
	}
	if rep.OK(strict) {
		fmt.Printf("\n━━━  verify passed (%d unpinned)\n", rep.Unpinned())
		return true
	}
	if rep.Unpinned() > 0 && !strict {
		fmt.Fprintf(os.Stderr, "  !  %d asset(s) without pinned sha256\n", rep.Unpinned())
	}
	fmt.Fprintf(os.Stderr, "\n━━━  verify failed: %d broken\n", rep.Failures())
	return false
}

// printSuggestions 输出 sha256 回填建议。
func printSuggestions(suggests []bundle.Suggestion) bool {
	if len(suggests) == 0 {
		fmt.Fprintln(os.Stderr, "  !  no archives to pin")
		return false
	}
	fmt.Println("      review the snippets below before writing assets.lock.toml")
	for _, s := range suggests {
		fmt.Printf("\n# ---- %s ([asset.platform.%s])\n  sha256 = %q\n", s.Ref, s.Platform, s.SHA256)
	}
	return true
}
