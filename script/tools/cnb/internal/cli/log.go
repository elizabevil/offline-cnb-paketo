package cli

import (
	"os"

	"log/slog"

	"learn-cnb-paketo/tools/cnb/internal/render"
)

// logr 是命令层共享的日志器（在 root PersistentPreRun 初始化）。
var logr = render.NewLogger(os.Stderr, render.LevelInfo)

// setupLogger 按 --log-level 初始化日志器。
func setupLogger(level string) {
	logr = render.NewLogger(os.Stderr, level)
}

var _ = slog.LevelInfo
