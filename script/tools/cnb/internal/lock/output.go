// Package lock 的输出（TSV 与 shell 赋值）。
package lock

import (
	"fmt"
	"maps"
	"slices"

	"io"
	"learn-cnb-paketo/tools/cnb/internal/errs"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/shell"
)

// WriteList 以 TSV 输出清单（id、version、kind、平台）。
func (f *File) WriteList(w io.Writer) {
	for _, a := range f.Assets {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.ID, a.Version, a.Unpack.Kind, strings.Join(slices.Sorted(maps.Keys(a.Platform)), ","))
	}
}

// Get 输出 shell 赋值，供 fetch.sh eval。字段名是契约，勿改。

// WriteShell 输出 shell 赋值语句，供脚本 eval。
func (f *File) WriteShell(w io.Writer, ref Ref, platform Platform) error {
	asset, err := Find(f, ref)
	if err != nil {
		return err
	}
	url, ok := asset.URL(platform)
	if !ok && !asset.IsLocal() {
		return &errs.Invalid{Field: "platform", Value: string(platform), Reason: fmt.Sprintf("not supported by %s (supported: %s)", ref, strings.Join(slices.Sorted(maps.Keys(asset.Platform)), ", "))}
	}
	dir := asset.Dir()
	if dir == "" {
		dir = "."
	}
	pairs := []struct{ key, value string }{
		{"ASSET_REF", asset.Ref()},
		{"ASSET_ID", asset.ID},
		{"ASSET_VERSION", asset.Version},
		{"ASSET_KIND", string(asset.Unpack.Kind)},
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
