package cli

import (
	"os"

	"log/slog"

	"learn-cnb-paketo/tools/cnb/internal/render"
)

// logr is the shared command logger, initialized in root PersistentPreRun.
var logr = render.NewLogger(os.Stderr, render.LevelInfo)

// setupLogger initializes the logger from --log-level.
func setupLogger(level string) {
	logr = render.NewLogger(os.Stderr, level)
}

var _ = slog.LevelInfo
