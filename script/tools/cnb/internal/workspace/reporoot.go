package workspace

// 本文件通过标记文件向上定位仓库根，避免依赖固定的目录层级。

import (
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// defaultMarkers are the files that identify a project root, in priority order.
var defaultMarkers = []string{"assets.lock.toml", ".git"}

// Markers returns the root markers, honouring CNB_ROOT_MARKERS ("a,b").
// Overriding them lets the tool run against any directory layout.
func Markers() []string {
	if env := os.Getenv(EnvRootMarkers); env != "" {
		var out []string
		for m := range strings.SplitSeq(env, ",") {
			if m = strings.TrimSpace(m); m != "" {
				out = append(out, m)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return defaultMarkers
}

// Find 从 start 目录向上查找含标记文件的目录。
func Find(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	markers := Markers()
	for {
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", &errs.NotFound{Kind: "repo root", Ref: start, Known: markers}
		}
		dir = parent
	}
}

// FindFromExecutable 以可执行文件所在目录为起点定位仓库根。
func FindFromExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return Find(filepath.Dir(exe))
}

// FindFromWorkingDir 以当前工作目录为起点定位仓库根。
func FindFromWorkingDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return Find(wd)
}
