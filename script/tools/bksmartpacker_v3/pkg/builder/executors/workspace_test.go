package executors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewWorkspaceTemporary(t *testing.T) {
	ws, err := NewWorkspace("")
	if err != nil {
		t.Fatal(err)
	}
	root := ws.Host
	for _, dir := range []string{ws.SourceDir(), ws.PlatformDir(), ws.LayersDir(), ws.RunDir(), ws.OutDir(), ws.TmpDir(), ws.AppDir()} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("missing dir %s: %v", dir, err)
		}
	}
	if ws.SourceContainerPath() != "/cnb/work/source/source.tgz" {
		t.Fatalf("source path = %q", ws.SourceContainerPath())
	}
	if err := ws.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("temporary workspace should be removed: %v", err)
	}
}

func TestNewWorkspaceConfiguredKeepsRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "work")
	ws, err := NewWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("configured workspace root should remain: %v", err)
	}
	entries, err := os.ReadDir(ws.OutDir())
	if err != nil {
		t.Fatalf("out dir should exist: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("out dir should be empty: %d entries", len(entries))
	}
}
