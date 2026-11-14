// Package errs 提供统一的错误包装与可判定的错误类型。
package errs

import (
	"errors"
	"fmt"
	"strings"
)

// Wrap 给错误加上操作上下文；err 为 nil 时返回 nil。
func Wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}

// Wrapf 同上，支持格式化上下文。
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// NotFound 表示引用不存在（物料、配置项等）。
type NotFound struct {
	Kind  string
	Ref   string
	Known []string
}

// Error 输出英文信息，便于日志与告警解析。
func (e *NotFound) Error() string {
	if len(e.Known) == 0 {
		return fmt.Sprintf("%s %q not found", e.Kind, e.Ref)
	}
	return fmt.Sprintf("%s %q not found (known: %s)", e.Kind, e.Ref, strings.Join(e.Known, ", "))
}

// IsNotFound 判断是否为 NotFound。
func IsNotFound(err error) bool {
	var target *NotFound
	return errors.As(err, &target)
}

// Invalid 表示输入不合法。
type Invalid struct {
	Field  string
	Value  string
	Reason string
}

// Error 输出英文信息。
func (e *Invalid) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("invalid %s %q", e.Field, e.Value)
	}
	return fmt.Sprintf("invalid %s %q: %s", e.Field, e.Value, e.Reason)
}

// Missing 表示必需内容缺失。
type Missing struct {
	What string
}

// Error 输出英文信息。
func (e *Missing) Error() string { return "missing " + e.What }
