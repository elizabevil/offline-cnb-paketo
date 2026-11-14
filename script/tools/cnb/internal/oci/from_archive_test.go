package oci

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"
)

func TestToLayout(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "img.tar")

	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(f)
	write := func(name string, data []byte) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	cfg := []byte(`{"architecture":"amd64","os":"linux"}`)
	write("abc.json", cfg)
	write("layer1/layer.tar", []byte("not-really-a-tar"))
	write("manifest.json", []byte(`[{"Config":"abc.json","RepoTags":["r:1"],"Layers":["layer1/layer.tar"]}]`))
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	layout := filepath.Join(dir, "layout")
	if err := ToLayout(archive, layout); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"index.json", "oci-layout"} {
		if _, err := os.Stat(filepath.Join(layout, want)); err != nil {
			t.Fatalf("缺少 %s", want)
		}
	}
	blobs, err := os.ReadDir(filepath.Join(layout, "blobs", "sha256"))
	if err != nil || len(blobs) < 3 {
		t.Fatalf("blob 数量不足: %v %v", blobs, err)
	}
}
