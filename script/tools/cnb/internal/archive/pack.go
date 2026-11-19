package archive

import (
	"archive/tar"
	"compress/gzip"
	"os"
)

// PackTarGz 把目录打包为 tar.gz（路径相对 srcDir，不含前导目录）。
func PackTarGz(srcDir, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	gz := gzip.NewWriter(out)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	return writeTree(tw, srcDir)
}
