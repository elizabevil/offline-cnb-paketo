package materials

import (
	"fmt"
	"strings"
)

// sourceErrors 汇总每个下载源的具体失败原因。
type sourceErrors []string

func (s *sourceErrors) add(url string, err error) {
	*s = append(*s, fmt.Sprintf("\n  - %s: %v", url, err))
}

func (s sourceErrors) String() string {
	if len(s) == 0 {
		return ""
	}
	return strings.Join(s, "")
}
