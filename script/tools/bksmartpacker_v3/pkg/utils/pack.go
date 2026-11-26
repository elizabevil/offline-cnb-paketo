package utils

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"time"
)

// PackDirTarGz 把目录打包为 tar.gz，属主固定 0:0、时间固定，产出可复现。
func PackDirTarGz(srcDir, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	gz := gzip.NewWriter(out)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		hdr.Uid, hdr.Gid = 0, 0
		hdr.Uname, hdr.Gname = "", ""
		hdr.ModTime = time.Unix(0, 0)
		hdr.AccessTime, hdr.ChangeTime = time.Time{}, time.Time{}
		switch {
		case info.IsDir():
			hdr.Name += "/"
			hdr.Mode = 0o755
		case info.Mode()&0o111 != 0:
			hdr.Mode = 0o755
		default:
			hdr.Mode = 0o644
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		_, err = io.Copy(tw, f)
		return err
	})
}
