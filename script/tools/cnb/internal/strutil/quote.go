package strutil

import "strings"

// 本文件提供 POSIX shell 字面量转义，供生成的脚本安全消费。

// QuoteShell 把字符串转成单引号包裹的 POSIX shell 字面量。
func QuoteShell(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
