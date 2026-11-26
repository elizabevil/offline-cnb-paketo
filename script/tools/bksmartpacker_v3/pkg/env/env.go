// Package env 提供环境变量名校验与 CNB platform env 目录写入。
package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	envDirName = "env"
	dirPerm    = 0o755
	filePerm   = 0o644
	envDirPerm = 0o755
)

// ValidKey 校验环境变量名：字母或下划线开头，后续可含数字。
func ValidKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// WritePlatform 把环境变量写成 <platformDir>/env/<KEY> 文件。
func WritePlatform(platformDir string, values map[string]string) error {
	envDir := filepath.Join(platformDir, envDirName)
	if err := os.MkdirAll(envDir, dirPerm); err != nil {
		return err
	}
	if err := os.Chmod(envDir, envDirPerm); err != nil {
		return err
	}
	for key, value := range values {
		if !ValidKey(key) {
			return fmt.Errorf("invalid env key %q", key)
		}
		if strings.ContainsAny(value, "\n\r") {
			return fmt.Errorf("env %s contains newline", key)
		}
		if err := os.WriteFile(filepath.Join(envDir, key), []byte(value), filePerm); err != nil {
			return err
		}
	}
	return nil
}
