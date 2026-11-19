// Package manifest 负责物料清单的解析、校验与访问。
package manifest

import (
	"strings"
)

// KnownPlatforms is the set of platforms the manifest accepts. It is derived
// from the platform whitelist so both cannot drift apart.
var KnownPlatforms = func() map[string]bool {
	out := make(map[string]bool, len(SupportedPlatforms()))
	for _, p := range SupportedPlatforms() {
		out[p] = true
	}
	return out
}()

// Ref 返回 id@version 形式的引用。
func (a Asset) Ref() Ref { return a.ID + "@" + a.Version }

// Dir 返回解包目录（相对 binaries）。
func (a Asset) Dir() string {
	dir := a.Unpack.Dir
	if dir == "" {
		dir = "."
	}
	return strings.ReplaceAll(dir, "{version}", a.Version)
}

// ExpectedPaths 返回解包后必须存在的完整相对路径。

// ExpectedPaths 返回解包后必须存在的完整相对路径。
// 支持 {version} 与 {arch} 占位符（{arch} 按平台渲染）。
func (a Asset) ExpectedPaths(platform Platform) []string {
	dir := a.Dir()
	arch := ArchOf(platform)
	out := make([]string, 0, len(a.Unpack.Expect))
	for _, e := range a.Unpack.Expect {
		rel := strings.ReplaceAll(e, "{version}", a.Version)
		rel = strings.ReplaceAll(rel, "{arch}", arch)
		if dir == "." {
			out = append(out, rel)
		} else {
			out = append(out, dir+"/"+rel)
		}
	}
	return out
}

// URL 返回展开后的下载地址。

// URL 返回平台下载地址；本地物料返回空。
func (a Asset) URL(platform Platform) (string, bool) {
	src, ok := a.Platform[string(platform)]
	if !ok {
		return "", false
	}
	return strings.ReplaceAll(src.URL, "{version}", a.Version), true
}

// IsLocal 表示该平台物料来自本地构建。
func (a Asset) IsLocal(platform Platform) bool {
	return a.Platform[string(platform)].Kind == KindLocal
}

// Load 读取并解析 lock。

// Platforms 返回清单声明的平台列表。
func (f *File) Platforms() []string {
	if len(f.Meta.Platforms) > 0 {
		return f.Meta.Platforms
	}
	return []string{PlatformAMD64, PlatformARM64}
}

// Select 返回指定 id 的物料；ids 为空返回全部。

// Mirrors 返回该平台声明的回退源。
func (a Asset) Mirrors(platform Platform) []string {
	return a.Platform[string(platform)].Mirrors
}
