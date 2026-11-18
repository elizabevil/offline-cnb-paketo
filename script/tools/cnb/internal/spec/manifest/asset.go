// Package lock 的物料访问方法。
package manifest

import ()

// KnownPlatforms 是受支持的平台集合。
var KnownPlatforms = map[string]bool{
	PlatformAMD64: true,
	PlatformARM64: true,
}

// Ref 返回 id@version 形式的引用。

// Ref 返回 id@version 形式的引用。
func (a Asset) Ref() Ref { return a.ID + "@" + a.Version }

// Dir 返回展开后的解压目录（相对 binaries/<platform>）。

// Dir 返回解包目录（相对 binaries）。
func (a Asset) Dir() string {
	dir := a.Unpack.Dir
	if dir == "" {
		dir = "."
	}
	return dir
}

// ExpectedPaths 返回解包后必须存在的完整相对路径。

// ExpectedPaths 返回解包后必须存在的完整相对路径。
func (a Asset) ExpectedPaths() []string {
	dir := a.Dir()
	out := make([]string, 0, len(a.Unpack.Expect))
	for _, e := range a.Unpack.Expect {
		if dir == "." {
			out = append(out, e)
		} else {
			out = append(out, dir+"/"+e)
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
	return src.URL, true
}

// IsLocal 表示该平台物料来自本地构建。
func (a Asset) IsLocal(platform Platform) bool {
	return a.Platform[string(platform)].Kind == "local"
}

// SHA256 返回该平台固定的哈希（可能为空）。
func (a Asset) SHA256(platform Platform) string {
	return a.Platform[string(platform)].SHA256
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

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// Size 返回该平台声明的大小（未声明时为 0）。
func (a Asset) Size(platform Platform) int64 {
	return a.Platform[string(platform)].Size
}
