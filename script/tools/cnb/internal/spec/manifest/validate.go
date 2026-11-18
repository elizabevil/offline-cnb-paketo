// Package lock 的结构校验。
package manifest

import (
	"fmt"
	"os"
	"strings"
)

// Validate 检查清单结构，返回错误条数。
//
//nolint:gocognit,cyclop,funlen // TODO(protocol-layers): 按 storage/asset/引用分节校验
func (f *File) Validate() int {
	var errs []string
	switch f.Storage.Layout {
	case "", LayoutArchFirst, LayoutKindFirst:
	default:
		errs = append(errs, fmt.Sprintf("[storage].layout must be %q or %q, got %q",
			LayoutArchFirst, LayoutKindFirst, f.Storage.Layout))
	}
	if f.Schema != 1 {
		errs = append(errs, fmt.Sprintf("schema 必须为 1，实际 %d", f.Schema))
	}
	metaPlatforms := f.Platforms()
	for _, p := range metaPlatforms {
		if !KnownPlatforms[p] {
			errs = append(errs, fmt.Sprintf("meta.platforms 含未知平台 %s", p))
		}
	}

	seen := map[string]bool{}
	byID := map[string]map[string]string{} // id -> dir -> version
	for _, a := range f.Assets {
		if a.ID == "" {
			errs = append(errs, "存在缺少 id 的 asset")
			continue
		}
		if seen[a.Ref()] {
			errs = append(errs, "重复的物料版本: "+a.Ref())
		}
		seen[a.Ref()] = true

		if a.Version == "" {
			errs = append(errs, a.ID+": 缺少 version")
		}
		switch a.Unpack.Kind {
		case KindTGZ, KindTarGz, KindZip, KindBin:
		default:
			errs = append(errs, fmt.Sprintf("%s: unpack.kind must be one of tgz / tar.gz / zip / bin, got %q", a.ID, a.Unpack.Kind))
		}
		if a.Unpack.Strip < 0 {
			errs = append(errs, fmt.Sprintf("%s: unpack.strip 不能为负（%d）", a.ID, a.Unpack.Strip))
		}
		if len(a.Unpack.Expect) == 0 {
			errs = append(errs, a.ID+": unpack.expect 不能为空（至少声明一个产物）")
		}
		if len(a.Platform) == 0 {
			errs = append(errs, a.ID+": 缺少 [asset.platform.<platform>] 表")
		}
		for _, want := range f.Meta.Platforms {
			if _, ok := a.Platform[want]; !ok {
				errs = append(errs, fmt.Sprintf("%s: 缺少 meta.platforms 声明的平台 %s", a.ID, want))
			}
		}
		for name, src := range a.Platform {
			switch src.Kind {
			case "", "http":
				if src.URL == "" {
					errs = append(errs, fmt.Sprintf("%s: %s 缺少 url", a.ID, name))
				}
			case "local":
				if src.Source == "" {
					errs = append(errs, fmt.Sprintf("%s: %s 缺少 source", a.ID, name))
				}
			default:
				errs = append(errs, fmt.Sprintf("%s: %s 未知 kind %q", a.ID, name, src.Kind))
			}
		}

		if byID[a.ID] == nil {
			byID[a.ID] = map[string]string{}
		}
		if prev, ok := byID[a.ID][a.Dir()]; ok && prev != a.Version {
			errs = append(errs, fmt.Sprintf(
				"%s 的版本 %s 与 %s 共用解压目录 %q，会互相覆盖；请为每个版本指定不同的 unpack.dir",
				a.ID, a.Version, prev, a.Dir()))
		}
		byID[a.ID][a.Dir()] = a.Version

		for p, src := range a.Platform {
			if !KnownPlatforms[p] {
				errs = append(errs, fmt.Sprintf("%s: 平台 %s 不受支持", a.ID, p))
				continue
			}
			if src.Kind == "local" {
				continue
			}
			if !strings.HasPrefix(src.URL, "http://") && !strings.HasPrefix(src.URL, "https://") {
				errs = append(errs, fmt.Sprintf("%s[%s]: url 必须是 http(s) 绝对地址，实际为 %q", a.ID, p, src.URL))
			}
			if src.SHA256 != "" && !isHex64(src.SHA256) {
				errs = append(errs, fmt.Sprintf("%s[%s]: sha256 格式非法", a.ID, p))
			}
		}
	}

	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  ✗ %s\n", e)
		}
		return len(errs)
	}
	fmt.Printf("  ✓ lock 结构合法：%d 个物料，%d 个平台\n", len(seen), len(metaPlatforms))
	return 0
}

// Find 支持 "id"（唯一或 default 版本）与 "id@version"。
