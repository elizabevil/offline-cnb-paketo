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

// vars 保存受控占位符的替换表。
type vars struct {
	replacer *strings.Replacer
	entries  []Placeholder
}

// Placeholder is one supported variable and where its value came from.
type Placeholder struct {
	Name   string
	Value  string
	Source string
}

// Table returns the supported placeholders in declaration order.
func (v vars) Table() []Placeholder { return v.entries }

// newVars 构建替换表：platform/arch/bundle + 每个已声明物料的 version。
func newVars(f *profile.File, lock *manifest.File, opts Options) (vars, error) {
	platform := manifest.Platform(f.Spec.Project.Platform)
	if _, err := manifest.ParsePlatform(string(platform)); err != nil {
		return vars{}, err
	}
	entries := []Placeholder{
		{"spec.project.platform", string(platform), "spec.project.platform"},
		{"runtime.project", opts.Project, "pipeline config directory"},
		{"spec.project.base", f.Spec.Project.Base[string(platform)], "spec.project.base[platform]"},
		{"derived.architecture", manifest.ArchOf(platform), "derived from spec.project.platform"},
		{"derived.container_platform", strings.Replace(string(platform), "-", "/", 1), `derived from spec.project.platform, "/" instead of "-" (for --platform)`},
		{"runtime.repo", opts.Repo, "workspace root (--repo / CNB_REPO_ROOT / executable path)"},
		{"runtime.materials", opts.Materials, "material manifest directory"},
		{"runtime.bundle", opts.Bundle, "bundle directory (--bundle or the lock directory)"},
	}
	for _, id := range slices.Sorted(maps.Keys(f.Spec.Assets)) {
		version := f.Spec.Assets[id].Version
		if version == "" {
			a, err := manifest.Find(lock, id)
			if err != nil {
				return vars{}, err
			}
			version = a.Version
		}
		entries = append(entries, Placeholder{"spec.assets." + id + ".version", version, "spec.assets." + id + ".version or lock default"})
	}
	pairs := make([]string, 0, len(entries)*2)
	for _, e := range entries {
		pairs = append(pairs, PlaceholderPrefix+e.Name+PlaceholderSuffix, e.Value)
	}
	return vars{replacer: strings.NewReplacer(pairs...), entries: entries}, nil
}

// expand 一次性替换已知占位符；出现未知占位符时报错。
func (v vars) expand(s string) (string, error) {
	out := v.replacer.Replace(s)
	if i := strings.Index(out, PlaceholderPrefix); i >= 0 {
		return "", &errs.Invalid{Field: "placeholder", Value: placeholderAt(out, i), Reason: "unknown placeholder (see `cnb pipeline variables <profile>`)"}
	}
	return out, nil
}

// placeholderAt 取出 out[i:] 处的占位符名称，仅用于错误信息。
func placeholderAt(out string, i int) string {
	body := out[i+len(PlaceholderPrefix):]
	name, _, ok := strings.Cut(body, PlaceholderSuffix)
	if !ok {
		return out[i:]
	}
	return PlaceholderPrefix + name + PlaceholderSuffix
}

// BundlePath 返回物料在 bundle 内的路径（供调用方拼接，避免重复规则）。
func BundlePath(bundle, platform, id, version string) string {
	return filepath.Join(bundle, platform, "binaries", id, version)
}
