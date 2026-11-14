package tomledit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `# 头部注释
[[asset]]
id = "jq"
version = "1.8.2"

  # 平台说明
  [asset.platform.linux-amd64]
  url = "https://example/jq"
  sha256 = ""
`

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "lock.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSetReplaces(t *testing.T) {
	p := writeTemp(t, sample)
	changed, err := Set(p, Section{Header: "[asset.platform.linux-amd64]", MatchID: "jq"}, "sha256", "abc")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), `sha256 = "abc"`) {
		t.Fatalf("未替换: %s", data)
	}
	if !strings.Contains(string(data), "# 头部注释") || !strings.Contains(string(data), "# 平台说明") {
		t.Fatal("注释被破坏")
	}
}

func TestSetInserts(t *testing.T) {
	p := writeTemp(t, sample)
	if _, err := Set(p, Section{Header: "[asset.platform.linux-amd64]", MatchID: "jq"}, "size", "123"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), `size = "123"`) {
		t.Fatalf("未插入: %s", data)
	}
}

func TestSetIdempotent(t *testing.T) {
	p := writeTemp(t, sample)
	if _, err := Set(p, Section{Header: "[asset.platform.linux-amd64]", MatchID: "jq"}, "sha256", "abc"); err != nil {
		t.Fatal(err)
	}
	changed, err := Set(p, Section{Header: "[asset.platform.linux-amd64]", MatchID: "jq"}, "sha256", "abc")
	if err != nil || changed {
		t.Fatalf("重复写入应无变化: changed=%v err=%v", changed, err)
	}
}
