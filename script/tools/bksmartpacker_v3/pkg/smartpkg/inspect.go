package smartpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// 包内约定文件名。
const (
	// VersionFile 标记层包格式（cnb-image-layers）。
	VersionFile = ".Version"
	// DescriptorFile 是 BK 应用描述。
	DescriptorFile = "app_desc.yaml"
	// LayerArchiveExt 是层包后缀。
	LayerArchiveExt = ".tgz"
)

// Report 是 s-mart 包的结构化检查结果。
type Report struct {
	Path    string   `json:"path" yaml:"path"`
	Size    int64    `json:"size" yaml:"size"`
	Version string   `json:"version" yaml:"version"`
	HasDesc bool     `json:"has_app_desc" yaml:"has_app_desc"`
	Files   []string `json:"files" yaml:"files"`
	Layers  []Layer  `json:"layers" yaml:"layers"`
}

// Layer 是包内单个层归档的摘要。
type Layer struct {
	Name    string   `json:"name" yaml:"name"`
	Size    int64    `json:"size" yaml:"size"`
	Entries []string `json:"entries" yaml:"entries"`
	// Compressed 表示层包是否为 gzip（否则是未压缩 tar）。
	Compressed bool `json:"compressed" yaml:"compressed"`
	// HasSource 表示层内含应用源码（打包时应排除）。
	HasSource bool `json:"has_source" yaml:"has_source"`
}

// Inspect 解析 s-mart 包：列出内容、读取 .Version、展开层包摘要。
func Inspect(pkgPath string) (Report, error) {
	info, err := os.Stat(pkgPath)
	if err != nil {
		return Report{}, err
	}
	rep := Report{Path: pkgPath, Size: info.Size()}

	f, err := os.Open(pkgPath)
	if err != nil {
		return Report{}, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return Report{}, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Report{}, err
		}
		name := hdr.Name
		rep.Files = append(rep.Files, name)
		switch {
		case name == VersionFile:
			data, _ := io.ReadAll(tr)
			rep.Version = strings.TrimSpace(string(data))
		case name == DescriptorFile:
			rep.HasDesc = true
		case strings.HasSuffix(name, LayerArchiveExt):
			data, err := io.ReadAll(tr)
			if err != nil {
				return Report{}, err
			}
			layer, err := inspectLayer(name, data)
			if err != nil {
				return Report{}, err
			}
			rep.Layers = append(rep.Layers, layer)
		}
	}
	return rep, nil
}

// inspectLayer 读取层包内的条目摘要。
// 包内条目可能是 gzip 压缩（.tgz），也可能是未压缩的 tar（docker save 产物），
// 按魔数判断，避免把 web.tgz 这类"名字是 tgz、内容是 tar"的文件误判。
func inspectLayer(name string, data []byte) (Layer, error) {
	layer := Layer{Name: path.Base(name)}
	var r io.Reader = bytes.NewReader(data)
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		gz, err := gzip.NewReader(r)
		if err != nil {
			return layer, fmt.Errorf("%s: gzip: %w", name, err)
		}
		defer gz.Close()
		r = gz
		layer.Compressed = true
	}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return layer, err
		}
		layer.Size += hdr.Size
		if len(layer.Entries) < 8 {
			layer.Entries = append(layer.Entries, hdr.Name)
		}
		if isSourcePath(hdr.Name) {
			layer.HasSource = true
		}
	}
	return layer, nil
}

// sourceMarkers 是应用源码的典型文件名；出现在层里说明打包范围过宽。
var sourceMarkers = []string{
	"package.json", "go.mod", "pyproject.toml", "requirements.txt", "Gemfile", "main.go",
}

// isSourcePath 判断路径是否像应用源码。
func isSourcePath(name string) bool {
	base := path.Base(name)
	for _, m := range sourceMarkers {
		if base == m {
			return true
		}
	}
	return false
}
