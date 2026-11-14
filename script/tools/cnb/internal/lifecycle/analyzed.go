package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/oci"
)

// analyzedFile 是 analyzer 产出的元数据文件名。
const analyzedFile = "analyzed.toml"

// writeAnalyzed 直接在磁盘上生成 analyzed.toml。
//
// 全部信息都来自磁盘：run image 的 reference 由配置给出，top-layer 取自
// layout 目录里 run image 的最后一层。因此不需要 analyzer，也不依赖 daemon。
func (c Config) writeAnalyzed() error {
	if c.RunImage == "" {
		return nil
	}
	dir, err := oci.LayoutImagePath(c.layoutDir(), c.RunImage)
	if err != nil {
		return err
	}
	top, err := oci.TopLayer(dir)
	if err != nil {
		fmt.Printf("  · run image layout %s: %v\n", dir, err)
		return err
	}
	fmt.Printf("  · run image top-layer: %s\n", shortDigest(top))
	fmt.Printf("  · run image reference: %s\n", c.referenceWithDigest())
	content := strings.Join([]string{
		"[run-image]",
		`  reference = "` + c.referenceWithDigest() + `"`,
		`  top-layer = "` + top + `"`,
		"",
	}, "\n")
	return os.WriteFile(filepath.Join(c.Layers, analyzedFile), []byte(content), 0o644)
}

// referenceWithDigest 返回 name@sha256:<digest> 形式的引用。
func (c Config) referenceWithDigest() string {
	name := c.RunImage
	if i := strings.LastIndex(name, "@"); i > 0 {
		name = name[:i]
	}
	return name + "@" + c.runImageDigest()
}

// runImageDigest 读取 layout 中 run image 的 manifest digest。
func (c Config) runImageDigest() string {
	dir, err := oci.LayoutImagePath(c.layoutDir(), c.RunImage)
	if err != nil {
		return ""
	}
	return oci.ManifestDigest(dir)
}

// shortDigest 只保留 digest 前 19 个字符，便于日志阅读。
func shortDigest(d string) string {
	if len(d) <= 19 {
		return d
	}
	return d[:19]
}
