package lifecycle

import (
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"learn-cnb-paketo/tools/cnb/internal/spec/imagefmt"
)

// standardPathDirs 是运行期 PATH 必须包含的目录。
var standardPathDirs = []string{
	"/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin",
}

// ensureStandardPath 保证镜像 config 的 PATH 含标准目录（缺则追加）。
// 通过 ConfigFile 读写，created / history 等字段保持不变。
func ensureStandardPath(img *imagefmt.Image) error {
	cf, err := img.V1.ConfigFile()
	if err != nil {
		return err
	}
	pathIndex := -1
	pathValue := ""
	for i, e := range cf.Config.Env {
		key, value, found := strings.Cut(e, "=")
		if found && key == "PATH" {
			pathIndex, pathValue = i, value
			break
		}
	}
	completed := withStandardDirs(pathValue)
	if pathIndex >= 0 {
		cf.Config.Env[pathIndex] = "PATH=" + completed
	} else {
		cf.Config.Env = append(cf.Config.Env, "PATH="+completed)
	}
	updated, err := mutate.ConfigFile(img.V1, cf)
	if err != nil {
		return err
	}
	img.V1 = updated
	return nil
}

// withStandardDirs 把缺失的标准目录追加到 PATH 末尾。
func withStandardDirs(path string) string {
	parts := strings.Split(path, ":")
	have := make(map[string]bool, len(parts))
	for _, p := range parts {
		if p != "" {
			have[p] = true
		}
	}
	for _, d := range standardPathDirs {
		if have[d] {
			continue
		}
		path = strings.TrimSuffix(path, ":") + ":" + d
	}
	return strings.TrimPrefix(path, ":")
}
