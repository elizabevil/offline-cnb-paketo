// Package lock 的读取与查找。
package lock

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// Load 读取并解析物料清单。
func Load(path string) (*File, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, &errs.NotFound{Kind: "lock file", Ref: path}
	}
	var f File
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil, readErr
	}
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, errs.Wrap("parse toml", err)
	}
	return &f, nil
}

// List 输出 TSV：id version kind platforms。

// Find 按 id 或 id@version 查找物料。
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
		return nil, &errs.Invalid{Field: "ref", Value: id, Reason: fmt.Sprintf("multiple versions, use id@version (available: %s)", strings.Join(versions, ", "))}
	}
}

// Platforms 返回平台矩阵。

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
