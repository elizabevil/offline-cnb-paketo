package checks

import "regexp"

var (
	metaIDRe   = regexp.MustCompile(`(?m)^\s*id\s*=\s*"([^"]+)"`)
	metaVerRe  = regexp.MustCompile(`(?m)^\s*version\s*=\s*"([^"]+)"`)
	metaAPIRe  = regexp.MustCompile(`(?m)^\s*api\s*=\s*"([^"]+)"`)
	metaHomeRe = regexp.MustCompile(`(?m)^\s*homepage\s*=\s*"([^"]+)"`)
	labelRe    = regexp.MustCompile(stackLabelPattern)
)

// firstMatch 返回首个捕获组，未命中返回空串。
func firstMatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}
