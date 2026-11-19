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

func TestEditVersionAcrossNestedTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "assets.toml")
	src := `[[asset]]
id = "a"
version = "1"

  [asset.info]
  name = "a"

  [asset.unpack]
  kind = "tgz"

[[asset]]
id = "b"
version = "2"
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// b 的 version 在下一个块，不能误改 a 的字段；a 有嵌套表，块不能被提前关闭。
	if _, err := EditVersion(path, "b", "2", "3"); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), `id = "a"`+"\n"+`version = "1"`) {
		t.Errorf("a 的 version 被改动:\n%s", out)
	}
	if !strings.Contains(string(out), `id = "b"`+"\n"+`version = "3"`) {
		t.Errorf("b 的 version 未更新:\n%s", out)
	}
}

func TestEditVersionMissingAsset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "assets.toml")
	if err := os.WriteFile(path, []byte("[[asset]]\nid = \"a\"\nversion = \"1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := EditVersion(path, "missing", "", "2")
	if changed || err == nil {
		t.Fatalf("期望 not found，得到 changed=%v err=%v", changed, err)
	}
	if _, ok := err.(*NotFoundInManifest); !ok {
		t.Fatalf("错误类型 = %T", err)
	}
}

func TestEditVersionKeepsMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "assets.toml")
	if err := os.WriteFile(path, []byte("[[asset]]\nid = \"a\"\nversion = \"1\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EditVersion(path, "a", "1", "2"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}
