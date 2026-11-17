// Package warn 提供协议解析过程中的“非致命问题”记录。
package warn

import "fmt"

// Warning 是一条解析告警（未知字段、可疑取值等）。
type Warning struct {
	// Path 是出问题的字段路径，如 "order[0].group[1].id"。
	Path string
	// Message 是说明。
	Message string
}

// String 便于日志输出。
func (w Warning) String() string { return fmt.Sprintf("%s: %s", w.Path, w.Message) }
