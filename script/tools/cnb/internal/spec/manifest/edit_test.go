package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditVersionPreservesRest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "assets.toml")
	src := `# 顶部注释
[[asset]]
id = "fnm"
version = "1.39.0"   # 版本
# 块内注释

  [asset.platform.linux-amd64]
  url = "https://x/v{version}/fnm.zip"
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := EditVersion(path, "fnm", "1.39.0", "1.40.0")
	if err != nil || !changed {
		t.Fatalf("编辑失败: changed=%v err=%v", changed, err)
	}
	out, _ := os.ReadFile(path)
	text := string(out)
	if !strings.Contains(text, `version = "1.40.0"   # 版本`) {
		t.Fatalf("version 行被破坏:\n%s", text)
	}
	if !strings.Contains(text, "# 顶部注释") || !strings.Contains(text, "# 块内注释") {
		t.Fatal("注释丢失")
	}
}
