package lock

import (
	"bytes"
	"os"

	"github.com/pelletier/go-toml/v2"

	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
)

// SetSHA 回填某平台物料的 sha256。
func SetSHA(path, refStr string, platform Platform, actual string) (bool, error) {
	return updateSource(path, refStr, platform, func(src *manifest.Source) {
		src.SHA256 = actual
	})
}

// SetSize 回填某平台物料的大小。
func SetSize(path, refStr string, platform Platform, size int64) (bool, error) {
	return updateSource(path, refStr, platform, func(src *manifest.Source) {
		src.Size = size
	})
}

// updateSource 读取整个 lock、修改目标平台的 Source、再序列化写回。
// 采用强类型解析，避免按行替换带来的类型与转义问题（注释会丢失）。
func updateSource(path, refStr string, platform Platform, mutate func(*manifest.Source)) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	file, _, err := manifest.Parse(data)
	if err != nil {
		return false, err
	}
	r, err := ParseRef(refStr)
	if err != nil {
		return false, err
	}
	found := false
	for i := range file.Assets {
		a := &file.Assets[i]
		if a.ID != r.ID || a.Version != r.Version {
			continue
		}
		src := a.Platform[string(platform)]
		mutate(&src)
		a.Platform[string(platform)] = src
		found = true
		break
	}
	if !found {
		return false, &notFoundRef{ref: refStr}
	}
	out, err := toml.Marshal(file)
	if err != nil {
		return false, err
	}
	if bytes.Equal(out, data) {
		return false, nil
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, path)
}

// notFoundRef 表示 lock 中没有对应物料。
type notFoundRef struct{ ref string }

func (e *notFoundRef) Error() string { return e.ref + " not found in lock" }
