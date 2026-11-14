// Package errs 提供统一的错误包装，避免各处重复格式化。
package errs

import "fmt"

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
