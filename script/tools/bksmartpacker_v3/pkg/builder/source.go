package builder

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const procfileName = "Procfile"

// BuildSourceTarball 把源码目录打成 tar.gz，并写入生成的 Procfile。
func BuildSourceTarball(src, procfileContent, destPath string) error {
	procfilePath := filepath.Join(src, procfileName)
	// 每次构建都重写 Procfile，避免不同模块共用源码目录时互相影响。
	if err := os.WriteFile(procfilePath, []byte(procfileContent), 0o755); err != nil {
		return errors.Wrap(err, "failed to write Procfile")
	}

	out, err := os.Create(destPath)
	if err != nil {
		return errors.Wrap(err, "failed to create source tarball")
	}
	defer func() { _ = out.Close() }()

	gz := gzip.NewWriter(out)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)

	if err := writeSourceEntries(tw, src); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return errors.Wrap(err, "failed to close tar writer")
	}
	if err := gz.Close(); err != nil {
		return errors.Wrap(err, "failed to close gzip writer")
	}
	if err := out.Close(); err != nil {
		return errors.Wrap(err, "failed to close source tarball")
	}
	return nil
}

// writeSourceEntries 以 src 为根写入归档；归档内路径不带 src 前缀。
func copyFileToTar(tw *tar.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(tw, f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func writeSourceEntries(tw *tar.Writer, src string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == src {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			hdr.Name += "/"
		}
		hdr.Uid, hdr.Gid = 0, 0
		hdr.Uname, hdr.Gname = "", ""
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFileToTar(tw, path)
	})
}
