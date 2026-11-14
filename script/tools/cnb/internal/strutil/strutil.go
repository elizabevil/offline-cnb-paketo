// Package strutil 提供少量字符串/切片工具，避免各包各写一份。
package strutil

// FirstNonEmpty 返回第一个非空字符串。
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Shorten 截断过长文本并加省略号。
func Shorten(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
