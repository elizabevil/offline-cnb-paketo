package lifecycle

import (
	"github.com/pelletier/go-toml/v2"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// Buildpack 是 buildpack.toml 的结构化表示（只含我们消费的字段）。
type Buildpack struct {
	API             string   `toml:"api"`
	BuildpackInfo   Info     `toml:"buildpack"`
	Order           []Group  `toml:"order"`
	MetadataSection Metadata `toml:"metadata"`
}

// Info 是 [buildpack] 段。
type Info struct {
	ID          string   `toml:"id"`
	Name        string   `toml:"name"`
	Version     string   `toml:"version"`
	Homepage    string   `toml:"homepage,omitempty"`
	SBOMFormats []string `toml:"sbom-formats,omitempty"`
}

// Metadata 是 [metadata] 段（只关心依赖声明）。
type Metadata struct {
	Dependencies []Dependency `toml:"dependencies"`
}

// Dependency 是一条依赖声明。
type Dependency struct {
	Name    string `toml:"name,omitempty"`
	Version string `toml:"version,omitempty"`
}

// ParseBuildpack 解析 buildpack.toml。
func ParseBuildpack(data []byte) (*Buildpack, []errs.Warning, error) {
	var bp Buildpack
	if err := toml.Unmarshal(data, &bp); err != nil {
		return nil, nil, err
	}
	return &bp, buildpackWarnings(data), nil
}

// buildpackWarnings 报告顶层未知键。
func buildpackWarnings(data []byte) []errs.Warning {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	known := map[string]bool{
		"api": true, "buildpack": true, "order": true, "metadata": true,
		"sbom-formats": true, "targets": true, "stacks": true, "licenses": true,
	}
	var out []errs.Warning
	for k := range raw {
		if !known[k] {
			out = append(out, errs.Warning{Path: k, Message: "unknown top-level key in buildpack.toml"})
		}
	}
	return out
}
