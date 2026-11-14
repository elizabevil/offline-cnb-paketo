// Package reporoot 的定位测试。
package reporoot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFind(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "assets.lock.toml"), []byte("schema = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o750); err != nil {
		t.Fatal(err)
	}
	got, err := Find(deep)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("期望 %s，实际 %s", root, got)
	}
}

func TestFindMissing(t *testing.T) {
	if _, err := Find(t.TempDir()); err == nil {
		t.Fatal("无标记时应报错")
	}
}
