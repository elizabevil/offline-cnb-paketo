// Package shell 提供 POSIX shell 字面量转义，供生成的脚本安全消费。
package shell

import "strings"

// Quote 把字符串转成单引号包裹的 POSIX shell 字面量。
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
