package fsutil

import (
	"io"
	"os"
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

// CopyExec 复制文件并设为可执行。
func CopyExec(src, dst string) error {
	if _, err := Copy(src, dst); err != nil {
		return err
	}
	return os.Chmod(dst, ExecMode)
}
