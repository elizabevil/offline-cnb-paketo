package manifest

import "testing"

func TestURLRendersVersion(t *testing.T) {
	a := Asset{
		ID: "fnm", Version: "1.40.0",
		Platform: map[string]Source{"linux-amd64": {URL: "https://x/v{version}/fnm.zip"}},
	}
	got, ok := a.URL(PlatformAMD64)
	if !ok || got != "https://x/v1.40.0/fnm.zip" {
		t.Fatalf("URL 渲染错误: %q ok=%v", got, ok)
	}
}

func TestDirRendersVersion(t *testing.T) {
	a := Asset{ID: "fnm", Version: "1.40.0", Unpack: Unpack{Dir: "fnm/{version}"}}
	if got := a.Dir(); got != "fnm/1.40.0" {
		t.Fatalf("Dir 渲染错误: %q", got)
	}
}

func TestURLWithoutPlaceholderUnchanged(t *testing.T) {
	a := Asset{ID: "x", Version: "9", Platform: map[string]Source{"linux-amd64": {URL: "https://x/fixed.zip"}}}
	got, _ := a.URL(PlatformAMD64)
	if got != "https://x/fixed.zip" {
		t.Fatalf("无占位符不应改写: %q", got)
	}
}
