// Package reporoot 通过标记文件向上定位仓库根，避免依赖固定的目录层级。
package reporoot

import (
	"fmt"
	"os"
	"path/filepath"
)

// Markers 是判定仓库根的标记文件（按优先级）。
var Markers = []string{"assets.lock.toml", ".git"}

// Find 从 start 目录向上查找含标记文件的目录。
func Find(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		for _, m := range Markers {
			if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("从 %s 向上未找到仓库根（标记: %v）", start, Markers)
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
