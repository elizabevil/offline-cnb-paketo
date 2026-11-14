package render

import (
	"os"
	"strings"
)

// ANSI 颜色码（无外部依赖，按需启用）。
const (
	ansiReset = "\x1b[0m"
	ansiRed   = "\x1b[31m"
	ansiGreen = "\x1b[32m"
	ansiYell  = "\x1b[33m"
	ansiCyan  = "\x1b[36m"
	ansiDim   = "\x1b[2m"
)

// Color 表示一种输出颜色。
type Color int

const (
	// ColorNone 不染色。
	ColorNone Color = iota
	// ColorRed 用于错误。
	ColorRed
	// ColorGreen 用于成功。
	ColorGreen
	// ColorYellow 用于警告。
	ColorYellow
	// ColorCyan 用于小节标题。
	ColorCyan
	// ColorDim 用于次要信息。
	ColorDim
)

// Enabled 判断是否应输出颜色：
// 仅在输出为重定向之外的终端、且未设置 NO_COLOR / TERM=dumb 时启用。
func Enabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// Paint 用指定颜色包裹文本；未启用颜色时原样返回。
func Paint(c Color, s string) string {
	if !Enabled() || c == ColorNone {
		return s
	}
	code := map[Color]string{
		ColorRed: ansiRed, ColorGreen: ansiGreen, ColorYellow: ansiYell,
		ColorCyan: ansiCyan, ColorDim: ansiDim,
	}[c]
	return code + s + ansiReset
}

// Success 渲染 "✓" 前缀。
func Success(s string) string { return Paint(ColorGreen, "✓") + " " + s }

// Failure 渲染 "✗" 前缀。
func Failure(s string) string { return Paint(ColorRed, "✗") + " " + s }

// Warn 渲染 "!" 前缀。
func Warn(s string) string { return Paint(ColorYellow, "!") + " " + s }

// Heading 渲染小节标题。
func Heading(s string) string { return Paint(ColorCyan, s) }
