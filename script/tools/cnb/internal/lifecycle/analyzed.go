package lifecycle

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// bareImageID 匹配 analyzed.toml 里只有镜像 ID 的 reference 行。
var bareImageID = regexp.MustCompile(`(?m)^(\s*reference\s*=\s*")([0-9a-f]{64})(")`)

// normalizeAnalyzed 把 analyzed.toml 中的裸镜像 ID 补成 name@digest 形式。
//
// analyzer 以 daemon 模式读取本地构建的 fake runner 时，只能拿到镜像 ID，
// 而 layout 模式的 exporter 要求 reference 形如 [path]@[digest]。
func (c Config) normalizeAnalyzed() error {
	if c.RunImage == "" {
		return nil
	}
	path := filepath.Join(c.Layers, "analyzed.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // 没有 analyzed.toml 时交给 exporter 自行报错
	}
	fixed := bareImageID.ReplaceAllString(string(data), "${1}"+c.refWithDigest()+"${3}")
	if fixed == string(data) {
		return nil
	}
	return os.WriteFile(path, []byte(fixed), 0o644)
}

// refWithDigest 返回 name@sha256: 前缀，digest 部分由正则捕获拼接。
func (c Config) refWithDigest() string {
	name := c.RunImage
	if i := strings.LastIndex(name, "@"); i > 0 {
		name = name[:i]
	}
	return name + "@sha256:"
}
