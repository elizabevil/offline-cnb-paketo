package archive

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/strutil"
)

// unpackName 去掉 strip 层前缀；不足 strip 层时丢弃。
func unpackName(name string, strip int) (string, bool) {
	clean := path.Clean(strings.TrimPrefix(name, "./"))
	for range strip {
		idx := strings.IndexByte(clean, '/')
		if idx < 0 {
			return "", false
		}
		clean = clean[idx+1:]
	}
	if clean == "" || clean == "." {
		return "", false
	}
	return clean, true
}

// selected 判断路径是否命中 include 通配（空表示全部）。
func selected(name string, include []string) bool {
	if len(include) == 0 {
		return true
	}
	for _, pat := range include {
		if strutil.Match(pat, name) {
			return true
		}
	}
	return false
}

// safeJoin 阻止路径逃逸。
func safeJoin(root, name string) (string, error) {
	target := filepath.Join(root, filepath.FromSlash(name))
	if rel, err := filepath.Rel(root, target); err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("unsafe path %q", name)
	}
	return target, nil
}
