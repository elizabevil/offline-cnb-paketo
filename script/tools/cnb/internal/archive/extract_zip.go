package archive

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

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
