package source

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestBuild(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "main.py"), []byte("print('ok')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "pkg", "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "source.tgz")
	if err := Build(src, "web: python main.py\n", dest); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gz.Close() }()

	entries := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			entries[hdr.Name] = string(data)
		}
	}
	if entries["Procfile"] != "web: python main.py\n" {
		t.Fatalf("unexpected Procfile: %q", entries["Procfile"])
	}
	if entries["main.py"] != "print('ok')\n" {
		t.Fatalf("unexpected main.py: %q", entries["main.py"])
	}
	if entries["pkg/a.txt"] != "a" {
		t.Fatalf("unexpected nested file: %q", entries["pkg/a.txt"])
	}
}
