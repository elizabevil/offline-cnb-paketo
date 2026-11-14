package lifecycle

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
)

// ExtractTarGz 把 tar.gz 解到 dst。
func ExtractTarGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := extractEntry(tr, h, dst); err != nil {
			return err
		}
	}
}

// extractEntry 解出一个条目。
func extractEntry(tr *tar.Reader, h *tar.Header, dst string) error {
	target := filepath.Join(dst, h.Name)
	switch h.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, fsutil.DirMode)
	case tar.TypeReg:
		return writeRegular(tr, h, target)
	default:
		return nil
	}
}

// writeRegular 写入普通文件并保留权限。
func writeRegular(tr *tar.Reader, h *tar.Header, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), fsutil.DirMode); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, tr); err != nil {
		_ = out.Close() // 复制失败时关闭错误无意义，返回原始错误
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(target, os.FileMode(h.Mode))
}
