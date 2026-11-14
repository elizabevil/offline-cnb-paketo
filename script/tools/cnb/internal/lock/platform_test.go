package lock

import "testing"

func TestParsePlatform(t *testing.T) {
	ok := []string{"linux-amd64", "linux-arm64", "darwin-arm64", "windows-amd64"}
	for _, s := range ok {
		if _, err := ParsePlatform(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	bad := []string{"", "linux", "linux-mips", "beos-amd64", "linux-amd64-extra"}
	for _, s := range bad {
		if _, err := ParsePlatform(s); err == nil {
			t.Fatalf("%q 应报错", s)
		}
	}
}

func TestPlatformParts(t *testing.T) {
	if !Platform("linux-amd64").IsLinux() || !Platform("darwin-arm64").IsDarwin() || !Platform("windows-amd64").IsWindows() {
		t.Fatal("OS 判定错误")
	}
	if ArchOf("darwin-arm64") != "arm64" {
		t.Fatal("ArchOf 错误")
	}
	if got := len(SupportedPlatforms()); got != 6 {
		t.Fatalf("支持平台数 = %d", got)
	}
}
