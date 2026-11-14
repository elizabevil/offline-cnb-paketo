package render

import (
	"io"
	"log/slog"
	"strings"
)

// 日志级别名。
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// NewLogger 构建统一格式的 slog 日志器。
func NewLogger(w io.Writer, level string) *slog.Logger {
	return slog.New(NewCompactHandler(w, ParseLevel(level)))
}

// ParseLevel 把级别名转为 slog.Level，未知值按 info 处理。
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
