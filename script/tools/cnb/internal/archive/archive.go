// Package archive 提供带路径安全校验的 tar.gz / zip 解包。
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	refutil "learn-cnb-paketo/tools/cnb/internal/ref"
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

	var files []string
	tr := tar.NewReader(gz)
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

// ExtractZip 解压 zip。
func ExtractZip(archive, dest string, opts Options) ([]string, error) {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()

	var files []string
	for _, f := range zr.File {
		name, ok := unpackName(f.Name, opts.Strip)
		if !ok || !selected(name, opts.Include) {
			continue
		}
		target, err := safeJoin(dest, name)
		if err != nil {
			return files, err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, dirMode); err != nil {
				return files, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
			return files, err
		}
		if err := unzipOne(f, target); err != nil {
			return files, err
		}
		files = append(files, name)
	}
	return files, nil
}

// unzipOne 写出单个 zip 条目。
func unzipOne(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// unpackName 去掉 strip 层前缀；不足 strip 层时丢弃。
func unpackName(name string, strip int) (string, bool) {
	clean := path.Clean(strings.TrimPrefix(name, "./"))
	for i := 0; i < strip; i++ {
		idx := strings.IndexByte(clean, '/')
		if idx < 0 {
			return "", false
		}
		clean = clean[idx+1:]
	}
	if clean == "" || clean == "." {
		return "", false
	}
	return clean, true
}

// selected 判断路径是否命中 include 通配（空表示全部）。
func selected(name string, include []string) bool {
	if len(include) == 0 {
		return true
	}
	for _, pat := range include {
		if refutil.Match(pat, name) {
			return true
		}
	}
	return false
}

// safeJoin 阻止路径逃逸。
func safeJoin(root, name string) (string, error) {
	target := filepath.Join(root, filepath.FromSlash(name))
	if rel, err := filepath.Rel(root, target); err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("unsafe path %q", name)
	}
	return target, nil
}

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
