package plan

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
	"learn-cnb-paketo/tools/cnb/internal/spec/profile"
)

// PlaceholderPrefix / Suffix 界定占位符边界。
const (
	PlaceholderPrefix = "${"
	PlaceholderSuffix = "}"
)

// vars 保存受控占位符的取值。
type vars struct {
	values map[string]string
}

// newVars 构建占位符表：platform/arch/bundle + 每个已声明物料的 version。
func newVars(f *profile.File, lock *manifest.File, opts Options) (vars, error) {
	platform := manifest.Platform(f.Spec.Project.Platform)
	if _, err := manifest.ParsePlatform(string(platform)); err != nil {
		return vars{}, err
	}
	v := vars{values: map[string]string{
		"platform": string(platform),
		"arch":     manifest.ArchOf(platform),
		"bundle":   opts.Bundle,
	}}
	for _, id := range slices.Sorted(maps.Keys(f.Spec.Assets)) {
		version := f.Spec.Assets[id].Version
		if version == "" {
			a, err := manifest.Find(lock, id)
			if err != nil {
				return vars{}, err
			}
			version = a.Version
		}
		v.values["asset."+id+".version"] = version
	}
	return v, nil
}

// expand 替换字符串中的占位符；未知占位符报错。
func (v vars) expand(s string) (string, error) {
	var b strings.Builder
	rest := s
	for {
		start := strings.Index(rest, PlaceholderPrefix)
		if start < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:start])
		body := rest[start+len(PlaceholderPrefix):]
		name, tail, ok := strings.Cut(body, PlaceholderSuffix)
		if !ok {
			return "", &errs.Invalid{Field: "placeholder", Value: s, Reason: "missing closing brace"}
		}
		value, known := v.values[name]
		if !known {
			return "", &errs.Invalid{Field: "placeholder", Value: name, Reason: "unknown placeholder"}
		}
		b.WriteString(value)
		rest = tail
	}
	return b.String(), nil
}

// sortedKeys 返回有序的 env 键。
func sortedKeys(m map[string]string) []string { return slices.Sorted(maps.Keys(m)) }

// BundlePath 返回物料在 bundle 内的路径（供调用方拼接，避免重复规则）。
func BundlePath(bundle, platform, id, version string) string {
	return filepath.Join(bundle, platform, "binaries", id, version)
}
