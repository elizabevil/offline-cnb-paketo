// Package fsutil 提供文件复制与哈希计算等通用操作。
package fsutil

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

const (
	// FileMode 是普通文件的默认权限（无类型常量，便于各处隐式转换）。
	FileMode = 0o644
	// DirMode 是目录的默认权限。
	DirMode = 0o755
	// ExecMode 是可执行文件的权限。
	ExecMode = 0o755
)

// Copy 复制文件并返回写入的字节数。
func Copy(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer func() { _ = out.Close() }()

	n, err := io.Copy(out, in)
	if err != nil {
		return n, err
	}
	return n, out.Close()
}

// CopyExec 复制文件并设置为可执行。
func CopyExec(src, dst string) error {
	if _, err := Copy(src, dst); err != nil {
		return err
	}
	return os.Chmod(dst, ExecMode)
}

// SHA256 返回文件的十六进制 SHA256。
func SHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
