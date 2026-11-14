package lifecycle

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Config 是 CNB lifecycle 一次执行的完整参数（来自环境变量或 flags）。
type Config struct {
	App         string
	Layers      string
	Platform    string
	Order       string // 源 order.toml；空则用 <cnb>/order.toml
	RunImage    string
	OutputImage string
	CacheImage  string
	SourceURL   string
	Required    string
	PlatformAPI string
	User        string // detect/build 使用的非 root 用户（默认 cnb）
	DryRun      bool
}

// FromEnv 读取 bksmartpacker / BK 平台约定的环境变量。
func FromEnv() Config {
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	return Config{
		App:         get("CNB_APP_DIR", "/app"),
		Layers:      get("CNB_LAYERS_DIR", "/layers"),
		Platform:    get("CNB_PLATFORM_DIR", "/platform"),
		Order:       "/cnb/order.toml",
		RunImage:    get("CNB_RUN_IMAGE", "fake-cnb-runner:latest"),
		OutputImage: os.Getenv("OUTPUT_IMAGE"),
		CacheImage:  os.Getenv("CACHE_IMAGE"),
		SourceURL:   os.Getenv("SOURCE_GET_URL"),
		Required:    os.Getenv("REQUIRED_BUILDPACKS"),
		PlatformAPI: get("CNB_PLATFORM_API", "0.15"),
		User:        get("CNB_RUN_USER", "cnb"),
	}
}

// Prepare 解包源码、生成过滤后的 order。
func (c Config) Prepare() error {
	if c.SourceURL != "" {
		src := strings.TrimPrefix(c.SourceURL, "file://")
		if src == c.SourceURL {
			return fmt.Errorf("暂不支持的 SOURCE_GET_URL: %s", c.SourceURL)
		}
		if err := os.MkdirAll(c.App, 0o755); err != nil {
			return err
		}
		if err := ExtractTarGz(src, c.App); err != nil {
			return fmt.Errorf("解包源码: %w", err)
		}
	}
	if c.Required != "" {
		out, err := FilterOrder(c.Order, c.Required)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(c.Layers, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(c.Layers, "order.toml"), out, 0o644)
	}
	return nil
}

// ExtractTarGz 把 tar.gz 解到 dst。
func ExtractTarGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
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
			os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0o755)
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
			os.Chmod(target, os.FileMode(h.Mode))
		}
	}
}
