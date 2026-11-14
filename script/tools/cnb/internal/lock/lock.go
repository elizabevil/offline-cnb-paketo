// Package lock 解析 assets.lock.toml：物料清单的唯一事实源。
//
// 每条物料按作用域分区，避免不同含义的字段混在一起：
//
//	[[asset]]
//	id / version / default        身份
//	[asset.info]                 元信息（给人看）
//	[asset.unpack]               解包行为（kind/strip/dir/expect）
//	[asset.platform.<platform>]  下载源（url/sha256）
package lock

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"learn-cnb-paketo/tools/cnb/internal/shell"
)

// KnownPlatforms 与 script/lib/common.sh 的 VALID_PLATFORMS 保持一致。
var KnownPlatforms = map[string]bool{
	PlatformAMD64: true,
	PlatformARM64: true,
}

type File struct {
	Schema int
	Meta   Meta    `toml:"meta"`
	Assets []Asset `toml:"asset"`
}

type Meta struct {
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	Provenance  string   `toml:"provenance"`
	CapturedAt  string   `toml:"captured_at"`
	Platforms   []string `toml:"platforms"`
}

// Asset 是一条物料。
// Local 表示该物料由本地构建产生，不走下载。
type Local struct {
	Source string `toml:"source"` // 相对仓库根的产物路径
	SHA256 string `toml:"sha256"` // 发布后回填
}

type Asset struct {
	ID      string `toml:"id"`
	Version string `toml:"version"`
	Default bool   `toml:"default"`

	Info     Info              `toml:"info"`
	Unpack   Unpack            `toml:"unpack"`
	Platform map[string]Source `toml:"platform"`
	Local    *Local            `toml:"local"`
}

// Info 是元信息，不参与构建。
type Info struct {
	Name     string `toml:"name"`
	Homepage string `toml:"homepage"`
	License  string `toml:"license"`
}

// Unpack 描述如何解包；Dir 支持 {version}。
type Unpack struct {
	Kind   string   `toml:"kind"`
	Strip  int      `toml:"strip"`
	Dir    string   `toml:"dir"`
	Expect []string `toml:"expect"`
}

// Source 是某平台的下载源。
type Source struct {
	URL    string `toml:"url"`
	SHA256 string `toml:"sha256"`
}

// Ref 返回 "id@version"。
func (a Asset) Ref() string { return a.ID + "@" + a.Version }

// Dir 返回展开后的解压目录（相对 binaries/<platform>）。
func (a Asset) Dir() string {
	dir := a.Unpack.Dir
	if dir == "" {
		dir = "."
	}
	return strings.ReplaceAll(dir, "{version}", a.Version)
}

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
func (a Asset) URL(platform string) (string, bool) {
	src, ok := a.Platform[platform]
	if !ok {
		return "", false
	}
	return strings.ReplaceAll(src.URL, "{version}", a.Version), true
}

// IsLocal 表示物料来自本地构建（[asset.local]）。
func (a Asset) IsLocal() bool { return a.Local != nil }

// SHA256 返回该平台固定的哈希（可能为空）；本地物料取 [asset.local].sha256。
func (a Asset) SHA256(platform string) string {
	if a.Local != nil {
		return a.Local.SHA256
	}
	return a.Platform[platform].SHA256
}

// Load 读取并解析 lock。
func Load(path string) (*File, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("未找到 lock 文件: %s", path)
	}
	var f File
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil, readErr
	}
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("TOML 解析失败: %w", err)
	}
	return &f, nil
}

// List 输出 TSV：id version kind platforms。
func (f *File) List(w io.Writer) {
	for _, a := range f.Assets {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.ID, a.Version, a.Unpack.Kind, strings.Join(sortedKeys(a.Platform), ","))
	}
}

// Get 输出 shell 赋值，供 fetch.sh eval。字段名是契约，勿改。
func (f *File) Get(w io.Writer, ref, platform string) error {
	asset, err := Find(f, ref)
	if err != nil {
		return err
	}
	url, ok := asset.URL(platform)
	if !ok && !asset.IsLocal() {
		return fmt.Errorf("物料 '%s' 不支持平台 '%s'（支持: %s）", ref, platform, strings.Join(sortedKeys(asset.Platform), ", "))
	}
	dir := asset.Dir()
	if dir == "" {
		dir = "."
	}
	pairs := []struct{ key, value string }{
		{"ASSET_REF", asset.Ref()},
		{"ASSET_ID", asset.ID},
		{"ASSET_VERSION", asset.Version},
		{"ASSET_KIND", asset.Unpack.Kind},
		{"ASSET_URL", url},
		{"ASSET_SHA256", asset.SHA256(platform)},
		{"ASSET_EXTRACT_TO", dir},
		{"ASSET_STRIP", fmt.Sprintf("%d", asset.Unpack.Strip)},
		{"ASSET_EXPECT", strings.Join(asset.ExpectedPaths(), " ")},
	}
	for _, p := range pairs {
		fmt.Fprintf(w, "%s=%s\n", p.key, shell.Quote(p.value))
	}
	return nil
}

// Validate 校验 lock 结构，返回错误条数。
func (f *File) Validate() int {
	var errs []string
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
		case "tgz", "tar.gz", "zip", "bin":
		default:
			errs = append(errs, fmt.Sprintf("%s: unpack.kind 必须是 tgz / tar.gz / zip / bin，实际 %q", a.ID, a.Unpack.Kind))
		}
		if a.Unpack.Strip < 0 {
			errs = append(errs, fmt.Sprintf("%s: unpack.strip 不能为负（%d）", a.ID, a.Unpack.Strip))
		}
		if len(a.Unpack.Expect) == 0 {
			errs = append(errs, a.ID+": unpack.expect 不能为空（至少声明一个产物）")
		}
		if a.Local != nil {
			if a.Local.Source == "" {
				errs = append(errs, a.ID+": [asset.local] 缺少 source")
			}
			// 本地物料不要求 platform 表
			continue
		}
		if len(a.Platform) == 0 {
			errs = append(errs, a.ID+": 缺少 [asset.platform.<platform>] 表")
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
			expanded := strings.ReplaceAll(src.URL, "{version}", a.Version)
			if !strings.HasPrefix(expanded, "http://") && !strings.HasPrefix(expanded, "https://") {
				errs = append(errs, fmt.Sprintf("%s[%s]: url 必须是 http(s) 绝对地址，展开后为 %q", a.ID, p, expanded))
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
func Find(f *File, ref string) (*Asset, error) {
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
		return nil, fmt.Errorf("未知物料 '%s'（已定义: %s）", ref, strings.Join(known, ", "))
	default:
		versions := make([]string, 0, len(matches))
		for _, i := range matches {
			versions = append(versions, f.Assets[i].Version)
		}
		return nil, fmt.Errorf("物料 '%s' 有多个版本，请用 id@version 指定（可选: %s）", id, strings.Join(versions, ", "))
	}
}

// Platforms 返回平台矩阵。
func (f *File) Platforms() []string {
	if len(f.Meta.Platforms) > 0 {
		return f.Meta.Platforms
	}
	return []string{PlatformAMD64, PlatformARM64}
}

// Select 返回指定 id 的物料；ids 为空返回全部。
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

func sortedKeys(m map[string]Source) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
