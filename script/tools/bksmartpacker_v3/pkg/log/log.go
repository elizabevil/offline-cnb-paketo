package log

import (
	"log/slog"
	"os"
	"strings"
)

// init 统一使用 slog 输出结构化文本日志。
// BKS_LOG_LEVEL 支持 debug/info/warn/error，默认 info。
func init() {
	level := slog.LevelInfo
	if v := strings.TrimSpace(os.Getenv("BKS_LOG_LEVEL")); v != "" {
		if err := level.UnmarshalText([]byte(v)); err != nil {
			level = slog.LevelInfo
		}
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02 15:04:05"))
			}
			return a
		},
	})
	slog.SetDefault(slog.New(handler))
}

// Info 输出 info 日志。
func Info(msg string, args ...any) { slog.Info(msg, args...) }

// Warn 输出 warning 日志。
func Warn(msg string, args ...any) { slog.Warn(msg, args...) }

// Error 输出 error 日志。
func Error(msg string, args ...any) { slog.Error(msg, args...) }

// Debug 输出 debug 日志。
func Debug(msg string, args ...any) { slog.Debug(msg, args...) }
