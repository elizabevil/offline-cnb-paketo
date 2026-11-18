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
}

// newVars 构建替换表：platform/arch/bundle + 每个已声明物料的 version。
func newVars(f *profile.File, lock *manifest.File, opts Options) (vars, error) {
	platform := manifest.Platform(f.Spec.Project.Platform)
	if _, err := manifest.ParsePlatform(string(platform)); err != nil {
		return vars{}, err
	}
	pairs := []string{
		PlaceholderPrefix + "platform" + PlaceholderSuffix, string(platform),
		PlaceholderPrefix + "arch" + PlaceholderSuffix, manifest.ArchOf(platform),
		PlaceholderPrefix + "oci_platform" + PlaceholderSuffix, strings.Replace(string(platform), "-", "/", 1),
		PlaceholderPrefix + "base_image" + PlaceholderSuffix, f.Spec.Project.Base[string(platform)],
		PlaceholderPrefix + "bundle" + PlaceholderSuffix, opts.Bundle,
		PlaceholderPrefix + "repo" + PlaceholderSuffix, opts.Repo,
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
		pairs = append(pairs, PlaceholderPrefix+"asset."+id+".version"+PlaceholderSuffix, version)
	}
	return vars{replacer: strings.NewReplacer(pairs...)}, nil
}

// expand 一次性替换已知占位符；出现未知占位符时报错。
func (v vars) expand(s string) (string, error) {
	out := v.replacer.Replace(s)
	if i := strings.Index(out, PlaceholderPrefix); i >= 0 {
		return "", &errs.Invalid{Field: "placeholder", Value: placeholderAt(out, i), Reason: "unknown placeholder"}
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
