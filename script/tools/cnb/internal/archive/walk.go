package archive

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
)

// writeTree 把 srcDir 下的内容按相对路径写入 tar 流。
// PackTar 与 PackTarGz 共用，避免两份相同的遍历逻辑。
func writeTree(tw *tar.Writer, srcDir string) error {
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
		return writeEntry(tw, path, rel, info)
	})
}

// writeEntry 写入单个文件或目录条目。
func writeEntry(tw *tar.Writer, path, rel string, info os.FileInfo) error {
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	normalizeHeader(hdr, info)
	hdr.Name = filepath.ToSlash(rel)
	if info.IsDir() {
		hdr.Name += "/"
		return tw.WriteHeader(hdr)
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(tw, f)
	return err
}
