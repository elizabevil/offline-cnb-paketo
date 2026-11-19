package materials

import (
	"fmt"
	"os"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// Check 校验物料并返回结构化结果（不打印）。
func Check(l *lock.File, bundleDir string, platform lock.Platform, ids []string) (Report, error) {
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
		report.Results = append(report.Results, checkOne(bundleDir, platform, a))
	}
	return report, nil
}

// checkOne 校验单个物料。
func checkOne(bundleDir string, platform lock.Platform, a *lock.Asset) CheckResult {
	if a.IsLocal(platform) {
		if pathsExist(binariesDir(bundleDir, platform), a.ExpectedPaths(platform)) {
			return CheckResult{Ref: a.Ref(), Status: StatusOK}
		}
		return CheckResult{Ref: a.Ref(), Status: StatusBroken,
			Problems: []string{"not published; run cnb publish " + a.ID}}
	}

	var problems []string
	archive := archivePath(bundleDir, platform, a)
	if _, err := os.Stat(archive); err != nil {
		problems = append(problems, "archive missing")
	} else if a.SHA256(platform) == "" {
		problems = append(problems, "sha256 not pinned")
	} else if got, err := fileSHA256(archive); err != nil {
		problems = append(problems, "hash error: "+err.Error())
	} else if got != a.SHA256(platform) {
		problems = append(problems, "hash mismatch")
	}

	if !isExtracted(bundleDir, platform, a) {
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
