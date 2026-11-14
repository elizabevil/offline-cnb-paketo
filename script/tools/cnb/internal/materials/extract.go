package materials

import (
	"fmt"
	"os"
	"path/filepath"

	"learn-cnb-paketo/tools/cnb/internal/archive"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// extractAsset 按 kind 解包，返回解出的文件列表。
func extractAsset(a *lock.Asset, src, binDir string) ([]string, error) {
	dest := filepath.Join(binDir, a.Dir())
	if err := os.MkdirAll(dest, fsutil.DirMode); err != nil {
		return nil, err
	}
	opts := archive.Options{Strip: a.Unpack.Strip, Include: a.Unpack.Include}
	switch a.Unpack.Kind {
	case lock.KindBin:
		name := a.Unpack.Expect[0]
		if err := fsutil.CopyFile(src, filepath.Join(binDir, name)); err != nil {
			return nil, err
		}
		return []string{name}, nil
	case lock.KindTGZ, lock.KindTarGz:
		return archive.ExtractTarGz(src, dest, opts)
	case lock.KindZip:
		return archive.ExtractZip(src, dest, opts)
	default:
		return nil, fmt.Errorf("unsupported kind %q", a.Unpack.Kind)
	}
}
