package manifest

import (
	"fmt"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// Find 按 id 或 id@version 查找物料；id 多版本时取 default = true 的那个。
func Find(f *File, ref Ref) (*Asset, error) {
	id, version, hasVersion := strings.Cut(ref, "@")
	var matches []int
	for i := range f.Assets {
		if f.Assets[i].ID != id {
			continue
		}
		if hasVersion && f.Assets[i].Version != version {
			continue
		}
		matches = append(matches, i)
	}
	if len(matches) > 1 && !hasVersion {
		var defaults []int
		for _, i := range matches {
			if f.Assets[i].Default {
				defaults = append(defaults, i)
			}
		}
		if len(defaults) == 1 {
			return &f.Assets[defaults[0]], nil
		}
	}
	switch len(matches) {
	case 1:
		return &f.Assets[matches[0]], nil
	case 0:
		known := make([]string, 0, len(f.Assets))
		for _, a := range f.Assets {
			known = append(known, a.Ref())
		}
		return nil, &errs.NotFound{Kind: "asset", Ref: ref, Known: known}
	default:
		versions := make([]string, 0, len(matches))
		for _, i := range matches {
			versions = append(versions, f.Assets[i].Version)
		}
		return nil, &errs.Invalid{Field: "ref", Value: id,
			Reason: fmt.Sprintf("multiple versions, use id@version (available: %s)", strings.Join(versions, ", "))}
	}
}

// Select 返回指定 id 集合的物料（空则全量）。
func (f *File) Select(ids []string) ([]Asset, error) {
	if len(ids) == 0 {
		return f.Assets, nil
	}
	out := make([]Asset, 0, len(ids))
	for _, id := range ids {
		a, err := Find(f, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, nil
}
