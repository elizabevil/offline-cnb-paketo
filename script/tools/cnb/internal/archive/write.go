package archive

import (
	"io"
	"os"
	"path/filepath"
)

// writeRegular 写入普通文件。
func writeRegular(r io.Reader, target string, mode int64) error {
	if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if mode&0o111 != 0 {
		mode = 0o755
	} else {
		mode = fileMode
	}
	return os.Chmod(target, os.FileMode(mode))
}
