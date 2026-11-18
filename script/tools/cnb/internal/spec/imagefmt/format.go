package imagefmt

import (
	"os"
	"path/filepath"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// Format 是一种镜像归档格式。
type Format interface {
	Name() string
	Detect(path string) bool
	Read(path string) (*Image, error)
	Write(img *Image, dest string) error
}

// formats 是内置实现，按探测顺序排列。
// OCI layout 优先：它可能同时带 docker 兼容的 manifest.json
var formats = []Format{ociLayout{}, dockerArchive{}}

// Detect 返回第一个匹配的格式。
func Detect(path string) (Format, error) {
	for _, f := range formats {
		if f.Detect(path) {
			return f, nil
		}
	}
	return nil, &errs.NotFound{Kind: "image format", Ref: path, Known: names()}
}

// Read 自动探测并解析镜像。
func Read(path string) (*Image, Format, error) {
	f, err := Detect(path)
	if err != nil {
		return nil, nil, err
	}
	img, err := f.Read(path)
	return img, f, err
}

// Write 按格式名写出镜像。
func Write(img *Image, format, dest string) error {
	for _, f := range formats {
		if f.Name() == format {
			return f.Write(img, dest)
		}
	}
	return &errs.Invalid{Field: "format", Value: format, Reason: "unknown image format"}
}

// names 返回支持的格式名。
func names() []string {
	out := make([]string, 0, len(formats))
	for _, f := range formats {
		out = append(out, f.Name())
	}
	return out
}

// exists 判断路径是否存在。
func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// join 拼接路径（便于统一改写基准目录）。
func join(elem ...string) string { return filepath.Join(elem...) }
