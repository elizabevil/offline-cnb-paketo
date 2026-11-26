package builder

import (
	"os"
	"path/filepath"

	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

const procfileName = "Procfile"

// BuildSourceTarball 把源码目录打成 tar.gz，并写入生成的 Procfile。
func BuildSourceTarball(src, procfileContent, destPath string) error {
	procfilePath := filepath.Join(src, procfileName)
	// 每次构建都重写 Procfile，避免不同模块共用源码目录时互相影响。
	if err := os.WriteFile(procfilePath, []byte(procfileContent), 0o755); err != nil {
		return errors.Wrap(err, "failed to write Procfile")
	}

	tarball := archiver.NewTarGz()
	entries, err := os.ReadDir(src)
	if err != nil {
		return errors.Wrap(err, "failed to read src")
	}
	paths := lo.Map(entries, func(item os.DirEntry, _ int) string {
		return filepath.Join(src, item.Name())
	})
	if err := tarball.Archive(paths, destPath); err != nil {
		return errors.Wrap(err, "failed to archive src")
	}
	return tarball.Close()
}
