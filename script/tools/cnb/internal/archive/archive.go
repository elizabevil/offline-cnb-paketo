// Package archive 提供带路径安全校验的 tar.gz / zip 解包。
package archive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"os"
	"path/filepath"
)

// 解包时的权限（与 fsutil 保持一致）。
const (
	dirMode  = fsutil.DirMode
	fileMode = fsutil.FileMode
)

// ExtractTarGz 解压 tar.gz。
func ExtractTarGz(archive, dest string, opts Options) ([]string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	return extractTarReader(tar.NewReader(gz), dest, opts)
}

// extractTarReader 从 tar.Reader 解包。
func extractTarReader(tr *tar.Reader, dest string, opts Options) ([]string, error) {
	var files []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return files, nil
		}
		if err != nil {
			return files, err
		}
		name, ok := unpackName(h.Name, opts.Strip)
		if !ok || !selected(name, opts.Include) {
			continue
		}
		target, err := safeJoin(dest, name)
		if err != nil {
			return files, err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, dirMode); err != nil {
				return files, err
			}
		case tar.TypeReg:
			if err := writeRegular(tr, target, h.Mode); err != nil {
				return files, err
			}
			files = append(files, name)
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
				return files, err
			}
			_ = os.Remove(target)
			if err := os.Symlink(h.Linkname, target); err != nil {
				return files, err
			}
		}
	}
}
