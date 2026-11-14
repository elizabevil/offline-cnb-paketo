package lifecycle

import (
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
)

// Prepare 解包源码、生成过滤后的 order。
func (c Config) Prepare() error {
	if err := c.extractSource(); err != nil {
		return err
	}
	return c.writeOrder()
}

// extractSource 从 SOURCE_GET_URL 解包源码到应用目录。
func (c Config) extractSource() error {
	if c.Build.SourceURL == "" {
		return nil
	}
	src := strings.TrimPrefix(c.Build.SourceURL, FileURLScheme)
	if src == c.Build.SourceURL {
		return &errs.Invalid{Field: "SOURCE_GET_URL", Value: c.Build.SourceURL, Reason: "only file:// is supported"}
	}
	if err := os.MkdirAll(c.Paths.App, fsutil.DirMode); err != nil {
		return err
	}
	if err := ExtractTarGz(src, c.Paths.App); err != nil {
		return errs.Wrap("解包源码", err)
	}
	return nil
}

// writeOrder 按 REQUIRED_BUILDPACKS 生成过滤后的 order.toml。
func (c Config) writeOrder() error {
	if c.Build.Required == "" {
		return nil
	}
	out, err := FilterOrder(c.Build.Order, c.Build.Required, c.Build.Aliases)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.Paths.Layers, fsutil.DirMode); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.Paths.Layers, OrderFileName), out, fsutil.FileMode)
}
