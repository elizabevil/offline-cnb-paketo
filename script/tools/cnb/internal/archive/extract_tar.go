package archive

import (
	"archive/tar"
	"os"
)

// ExtractTar 把未压缩 tar 解到 dest（不 strip、不过滤）。
func ExtractTar(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = extractTarReader(tar.NewReader(f), dest, Options{})
	return err
}
