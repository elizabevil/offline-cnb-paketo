package smartpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

// 包内约定文件名。
const (
	// VersionFile 标记层包格式（cnb-image-layers）。
	VersionFile = ".Version"
	// DescriptorFile 是 BK 应用描述。
	DescriptorFile = "app_desc.yaml"
	// LayerArchiveExtTgz 是 gzip 层包后缀。
	LayerArchiveExtTgz = ".tgz"
	// LayerArchiveExtTar 是未压缩层包后缀（layout / docker save）。
	LayerArchiveExtTar = ".tar"
	// ArtifactFile 是 v2 的构建映射文件。
	ArtifactFile = "artifact.json"
)

// Report 是 s-mart 包的结构化检查结果。
type Report struct {
	Path    string   `json:"path" yaml:"path"`
	Size    int64    `json:"size" yaml:"size"`
	Version string   `json:"version" yaml:"version"`
	HasDesc bool     `json:"has_app_desc" yaml:"has_app_desc"`
	Files   []string `json:"files" yaml:"files"`
	// HasArtifact 表示包内存在 v2 的 artifact.json
	HasArtifact bool `json:"has_artifact" yaml:"has_artifact"`
	// ArtifactModules 是 artifact.json 中声明的模块名
	ArtifactModules []string `json:"artifact_modules,omitempty" yaml:"artifact_modules,omitempty"`
	Layers          []Layer  `json:"layers" yaml:"layers"`
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
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return Report{}, err
	}
	defer func() { _ = gz.Close() }()

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
		case name == ArtifactFile:
			rep.HasArtifact = true
			data, _ := io.ReadAll(tr)
			var artifact struct {
				AppArtifacts map[string]struct{} `json:"app_artifacts"`
			}
			if json.Unmarshal(data, &artifact) == nil {
				for module := range artifact.AppArtifacts {
					rep.ArtifactModules = append(rep.ArtifactModules, module)
				}
				sort.Strings(rep.ArtifactModules)
			}
		case isLayerArchive(name):
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
		defer func() { _ = gz.Close() }()
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

// sourceMarkers 是应用源码根目录的典型文件。
// 注意：这是启发式检查，只覆盖常见语言，不能证明层里一定没有源码。
var sourceMarkers = map[string]struct{}{
	"package.json":      {},
	"package-lock.json": {},
	"yarn.lock":         {},
	"pnpm-lock.yaml":    {},
	"go.mod":            {},
	"go.sum":            {},
	"pyproject.toml":    {},
	"requirements.txt":  {},
	"Pipfile":           {},
	"Pipfile.lock":      {},
	"poetry.lock":       {},
	"setup.py":          {},
	"setup.cfg":         {},
	"manage.py":         {},
	"app.py":            {},
	"main.py":           {},
	"Gemfile":           {},
	"Gemfile.lock":      {},
	"main.go":           {},
	"pom.xml":           {},
	"build.gradle":      {},
	"build.gradle.kts":  {},
	"settings.gradle":   {},
	"Cargo.toml":        {},
	"Cargo.lock":        {},
	"composer.json":     {},
	"composer.lock":     {},
	"mix.exs":           {},
	"mix.lock":          {},
	"index.js":          {},
	"runtime.txt":       {},
}

// isLayerArchive 判断外层条目是否为一个镜像层包。
func isLayerArchive(name string) bool {
	switch path.Ext(name) {
	case LayerArchiveExtTgz, LayerArchiveExtTar:
		return true
	default:
		return false
	}
}

// isSourcePath 判断路径是否像应用源码。
//
// 只检查 /app 或 /workspace 的根文件，避免把依赖目录里的 setup.py、
// 测试夹具里的 package.json 误判为源码；仍无法覆盖所有语言，仅作告警。
func isSourcePath(name string) bool {
	clean := strings.TrimPrefix(path.Clean("/"+name), "/")
	var rel string
	switch {
	case strings.HasPrefix(clean, "app/"):
		rel = strings.TrimPrefix(clean, "app/")
	case strings.HasPrefix(clean, "workspace/"):
		rel = strings.TrimPrefix(clean, "workspace/")
	default:
		return false
	}
	if rel == "" || strings.Contains(rel, "/") {
		return false
	}
	if _, ok := sourceMarkers[rel]; ok {
		return true
	}
	return strings.HasSuffix(rel, ".csproj")
}
