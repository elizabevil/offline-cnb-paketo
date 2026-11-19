package archive

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackTarExtractTarRoundTrip(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "index.json"), `{"schemaVersion":2}`)
	writeFile(t, filepath.Join(src, "blobs", "sha256", "abc"), "layer")

	tarPath := filepath.Join(t.TempDir(), "image.tar")
	if err := PackTar(src, tarPath); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := ExtractTar(tarPath, dst); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"index.json", filepath.Join("blobs", "sha256", "abc")} {
		got, err := os.ReadFile(filepath.Join(dst, name))
		if err != nil {
			t.Fatalf("缺少 %s: %v", name, err)
		}
		want, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("%s 内容不一致: %q != %q", name, got, want)
		}
	}
}

func TestPackTarIsUncompressed(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "a.txt"), "x")
	tarPath := filepath.Join(t.TempDir(), "image.tar")
	if err := PackTar(src, tarPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2 || (data[0] == 0x1f && data[1] == 0x8b) {
		t.Fatal("PackTar 不应输出 gzip")
	}
}

func TestPackTarDeterministic(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "b.txt"), "b")
	writeFile(t, filepath.Join(src, "a.txt"), "a")
	one := filepath.Join(t.TempDir(), "1.tar")
	two := filepath.Join(t.TempDir(), "2.tar")
	if err := PackTar(src, one); err != nil {
		t.Fatal(err)
	}
	if err := PackTar(src, two); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(one)
	b, _ := os.ReadFile(two)
	if !bytes.Equal(a, b) {
		t.Fatal("相同内容应产出相同 tar")
	}
}
