package render

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// CompactHandler 输出 `• msg key=value …`，比 slog 默认格式更适合 CLI。
type CompactHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
}

func NewCompactHandler(w io.Writer, level slog.Level) *CompactHandler {
	return &CompactHandler{w: w, level: level}
}

func (h *CompactHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *CompactHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString("• ")
	b.WriteString(r.Message)
	write := func(k string, v slog.Value) {
		b.WriteString(" ")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v.String())
	}
	for _, a := range h.attrs {
		write(a.Key, a.Value)
	}
	r.Attrs(func(a slog.Attr) bool { write(a.Key, a.Value); return true })
	if r.Level >= slog.LevelWarn {
		b.WriteString(" level=")
		b.WriteString(r.Level.String())
	}
	_, err := fmt.Fprintln(h.w, b.String())
	return err
}

func (h *CompactHandler) WithAttrs(as []slog.Attr) slog.Handler {
	return &CompactHandler{w: h.w, level: h.level, attrs: append(append([]slog.Attr{}, h.attrs...), as...)}
}

func (h *CompactHandler) WithGroup(string) slog.Handler { return h }
