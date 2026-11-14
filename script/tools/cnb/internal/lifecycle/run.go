package lifecycle

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
)

// Bins 指定四个 lifecycle 二进制的位置（由调用方决定）。
type Bins struct {
	Detector string
	Builder  string
	Analyzer string
	Exporter string
}

// Config 是一次 lifecycle 执行的全部输入；不含任何项目假设。
type Config struct {
	Bins         Bins
	App          string
	Layers       string
	Platform     string
	Order        string // 源 order.toml；空则用 <cnb>/order.toml
	RunImage     string
	OutputImage  string
	CacheImage   string
	SourceURL    string
	Required     string
	PlatformAPI  string
	User         string
	ExportLayout bool
	LayoutDir    string
	LayerArchive string
	// Aliases 可把外部平台的 buildpack 名映射到 order 里的 id（可为空）。
	Aliases map[string]string
	// RunImageArchive 是 run image 的 docker-archive 路径（由平台侧预先落盘）。
	// 非空时不再调用运行时，完全磁盘驱动。
	RunImageArchive string
	// Runtime 是容器运行时命令（默认依次尝试 docker、podman）。
	Runtime string
	// IncludeSource 为真时把应用源码一并打进层包（默认否，只要构建产物）。
	IncludeSource bool
	// Out 是进度输出目标（默认 os.Stdout）。
	Out    io.Writer // detect/build 使用的非 root 用户（默认 cnb）
	DryRun bool
}

// Prepare 解包源码、生成过滤后的 order。
func (c Config) Prepare() error {
	if err := c.extractSource(); err != nil {
		return err
	}
	return c.writeOrder()
}

// extractSource 从 SOURCE_GET_URL 解包源码到应用目录。
func (c Config) extractSource() error {
	if c.SourceURL == "" {
		return nil
	}
	src := strings.TrimPrefix(c.SourceURL, FileURLScheme)
	if src == c.SourceURL {
		return fmt.Errorf("暂不支持的 SOURCE_GET_URL: %s", c.SourceURL)
	}
	if err := os.MkdirAll(c.App, fsutil.DirMode); err != nil {
		return err
	}
	if err := ExtractTarGz(src, c.App); err != nil {
		return errs.Wrap("解包源码", err)
	}
	return nil
}

// writeOrder 按 REQUIRED_BUILDPACKS 生成过滤后的 order.toml。
func (c Config) writeOrder() error {
	if c.Required == "" {
		return nil
	}
	out, err := FilterOrder(c.Order, c.Required, c.Aliases)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.Layers, fsutil.DirMode); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.Layers, OrderFileName), out, fsutil.FileMode)
}

// ExtractTarGz 把 tar.gz 解到 dst。
func ExtractTarGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dst, h.Name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, fsutil.DirMode); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), fsutil.DirMode); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close() // 复制失败时关闭错误无意义，返回原始错误
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			if err := os.Chmod(target, os.FileMode(h.Mode)); err != nil {
				return err
			}
		}
	}
}
