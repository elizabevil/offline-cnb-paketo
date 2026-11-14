package apparchive

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"
)

func TestBuild(t *testing.T) {
	app := t.TempDir()
	if err := os.WriteFile(filepath.Join(app, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "out.tar")
	if err := Build(app, "", "demo/app:1", "amd64", dest, true); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	names := map[string]bool{}
	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		names[h.Name] = true
	}
	if !names["manifest.json"] {
		t.Fatalf("缺少 manifest.json: %v", names)
	}
}
