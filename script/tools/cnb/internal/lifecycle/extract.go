package lifecycle

import "learn-cnb-paketo/tools/cnb/internal/archive"

// ExtractTarGz 把 tar.gz 解到 dst（应用源码包，不做 strip 与过滤）。
func ExtractTarGz(src, dst string) error {
	_, err := archive.ExtractTarGz(src, dst, archive.Options{})
	return err
}
