package materials

import "learn-cnb-paketo/tools/cnb/internal/lock"

// countStatus counts materials with the given status.
func (r Report) countStatus(want Status) int {
	n := 0
	for _, res := range r.Results {
		if res.Status == want {
			n++
		}
	}
	return n
}

// Status 是单个物料的校验结论。
type Status string

const (
	// StatusOK 表示物料完整可用。
	StatusOK Status = "ok"
	// StatusUnpinned 表示可用但 sha256 未固定。
	StatusUnpinned Status = MarkerUnpinned
	// StatusBroken 表示缺失或哈希不符。
	StatusBroken Status = "broken"
)

// CheckResult 是单个物料的校验条目。
type CheckResult struct {
	Ref      lock.Ref
	Status   Status
	Problems []string
}

// Report 是一次校验的完整结果。
type Report struct {
	Platform string
	Results  []CheckResult
}

// Failures returns the number of broken materials.
func (r Report) Failures() int {
	return r.countStatus(StatusBroken)
}

// Unpinned returns the number of materials without a pinned hash.
func (r Report) Unpinned() int {
	return r.countStatus(StatusUnpinned)
}

// OK 判断校验是否通过：无 broken；未固定项在 strict 下视为失败。
func (r Report) OK(strict bool) bool {
	if r.Failures() > 0 {
		return false
	}
	return !strict || r.Unpinned() == 0
}

// Suggestion 是一条待回填的 sha256 建议。
type Suggestion struct {
	Ref      lock.Ref
	Platform lock.Platform
	SHA256   string
}
