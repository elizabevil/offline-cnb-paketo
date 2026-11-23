package materials

import (
	"fmt"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"os"

	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
)

// Check 校验物料并返回结构化结果（不打印）。
func Check(l *lock.File, bundleDir string, platform lock.Platform, ids []string, lockFile *manifest.LockFile, quick bool) (Report, error) {
	if !lock.KnownPlatforms[string(platform)] {
		return Report{}, fmt.Errorf("unsupported platform: %s", platform)
	}
	assets, err := l.Select(ids)
	if err != nil {
		return Report{}, err
	}

	report := Report{Platform: string(platform)}
	for i := range assets {
		a := &assets[i]
		report.Results = append(report.Results, checkOne(bundleDir, platform, a, lockFile, quick))
	}
	return report, nil
}

// checkOne 校验单个物料。
func checkOne(bundleDir string, platform lock.Platform, a *lock.Asset, lockFile *manifest.LockFile, quick bool) CheckResult {
	opts := Options{LockFile: lockFile, Platform: platform}
	locked, _ := lockSource(a, platform, opts)
	if a.IsLocal(platform) {
		if expectOK(bundleDir, platform, a) {
			return CheckResult{Ref: a.Ref(), Status: StatusOK}
		}
		return CheckResult{Ref: a.Ref(), Status: StatusBroken,
			Problems: []string{"not published; run cnb publish " + a.ID}}
	}

	var problems []string
	archive := archivePath(bundleDir, platform, a)
	if _, err := os.Stat(archive); err != nil {
		problems = append(problems, "archive missing")
	} else if locked.SHA256 == "" {
		problems = append(problems, "sha256 not pinned")
	} else if quick {
		if fi, err := os.Stat(archive); err != nil {
			problems = append(problems, "archive missing")
		} else if locked.Size > 0 && fi.Size() != locked.Size {
			problems = append(problems, "size mismatch")
		}
	} else if got, err := fsutil.SHA256(archive); err != nil {
		problems = append(problems, "hash error: "+err.Error())
	} else if got != locked.SHA256 {
		problems = append(problems, "hash mismatch")
	}

	if !isExtracted(bundleDir, platform, a, locked.SHA256) {
		problems = append(problems, "not extracted or incomplete")
	}
	return classify(a.Ref(), problems)
}

// classify 根据问题列表判定状态。
func classify(ref lock.Ref, problems []string) CheckResult {
	if len(problems) == 0 {
		return CheckResult{Ref: ref, Status: StatusOK}
	}
	if len(problems) == 1 && problems[0] == "sha256 not pinned" {
		return CheckResult{Ref: ref, Status: StatusUnpinned, Problems: problems}
	}
	return CheckResult{Ref: ref, Status: StatusBroken, Problems: problems}
}
