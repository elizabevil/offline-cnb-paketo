package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testAliases = map[string]string{
	"bk-buildpack-go":     "paketo-buildpacks/go",
	"bk-buildpack-nodejs": "paketo-buildpacks/nodejs",
}

const sample = `[[order]]
  [[order.group]]
    id = "paketo-buildpacks/go"
    version = "4.23.3"

[[order]]
  [[order.group]]
    id = "paketo-buildpacks/nodejs"
    version = "10.15.0"

[[order]]
  [[order.group]]
    id = "paketo-buildpacks/python"
    version = "2.68.0"
`

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "order.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFilterOrderByAlias(t *testing.T) {
	src := writeTemp(t, sample)
	out, err := FilterOrder(src, "oci-embedded bk-buildpack-go ... v191", testAliases)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "paketo-buildpacks/go") {
		t.Fatalf("未保留 go: %s", out)
	}
	if strings.Contains(string(out), "nodejs") || strings.Contains(string(out), "python") {
		t.Fatalf("误保留其它 buildpack: %s", out)
	}
}

func TestFilterOrderByRawID(t *testing.T) {
	src := writeTemp(t, sample)
	out, err := FilterOrder(src, "x paketo-buildpacks/nodejs y v1", testAliases)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "nodejs") || strings.Contains(string(out), "go\"") {
		t.Fatalf("过滤结果不符: %s", out)
	}
}

func TestFilterOrderEmptyKeepsAll(t *testing.T) {
	src := writeTemp(t, sample)
	out, err := FilterOrder(src, "", testAliases)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "go") || !strings.Contains(string(out), "python") {
		t.Fatal("为空时应原样返回")
	}
}

func TestFilterOrderNoMatch(t *testing.T) {
	src := writeTemp(t, sample)
	if _, err := FilterOrder(src, "x bk-buildpack-ruby y v1", testAliases); err == nil {
		t.Fatal("无交集应报错")
	}
}
