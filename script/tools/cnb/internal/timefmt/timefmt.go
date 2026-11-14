// Package timefmt 统一时间格式：UTC + RFC3339（以 Z 结尾）。
package timefmt

import "time"

// Layout 是仓库统一的输出格式。
const Layout = time.RFC3339

// Now 返回当前 UTC 时间（截断到秒）。
func Now() time.Time { return time.Now().UTC().Truncate(time.Second) }

// Format 把时间格式化为 UTC + Z。
func Format(t time.Time) string { return t.UTC().Format(Layout) }

// String 是 Format(Now()) 的简写。
func String() string { return Format(Now()) }
