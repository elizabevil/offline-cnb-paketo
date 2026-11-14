// Package lock 的输出（TSV 与 shell 赋值）。
package lock

import (
	"fmt"
	"io"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/shell"
)

// List 输出 TSV 清单（id、version、kind、平台）。
func (f *File) List(w io.Writer) {
	for _, a := range f.Assets {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.ID, a.Version, a.Unpack.Kind, strings.Join(sortedKeys(a.Platform), ","))
	}
}

// Get 输出 shell 赋值，供 fetch.sh eval。字段名是契约，勿改。

// Get 输出 shell 赋值，供 fetch.sh eval。
func (f *File) Get(w io.Writer, ref Ref, platform Platform) error {
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
