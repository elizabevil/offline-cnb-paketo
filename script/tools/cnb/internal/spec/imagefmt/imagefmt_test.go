package imagefmt

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// writeLayerTar 生成一个只含单个文件的层 tar。
func writeLayerTar(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "layer.tar")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	tw := tar.NewWriter(f)
	body := []byte("hello")
	if err := tw.WriteHeader(&tar.Header{Name: "app/hello.txt", Mode: 0o644, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// sampleImage 构造带 created / history / 标签的镜像。
func sampleImage(t *testing.T, dir string) *Image {
	t.Helper()
	layer, err := tarball.LayerFromFile(writeLayerTar(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	img, err = mutate.ConfigFile(img, &v1.ConfigFile{
		Created:      v1.Time{Time: created},
		Architecture: "amd64",
		OS:           "linux",
		Config: v1.Config{
			Entrypoint: []string{"/cnb/process/web"},
			WorkingDir: "/app",
			Env:        []string{"HOME=/app"},
		},
		History: []v1.History{{Created: v1.Time{Time: created}, CreatedBy: "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &Image{V1: img, RepoTags: []string{"bk_cmdb_saas/web:1.0"}}
}

func TestLayoutRoundTrip(t *testing.T) {
	src := t.TempDir()
	img := sampleImage(t, src)
	if err := img.Write(FormatOCILayout, filepath.Join(src, "oci")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{fileIndexJSON, fileOCILayout, "manifest.json", "repositories"} {
		if _, err := os.Stat(filepath.Join(src, "oci", name)); err != nil {
			t.Fatalf("缺少 %s: %v", name, err)
		}
	}

	got, format, err := Open(filepath.Join(src, "oci"))
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatOCILayout {
		t.Fatalf("格式探测错误: %s", format)
	}
	assertSameConfig(t, img.V1, got.V1)
	if len(got.RepoTags) == 0 || got.RepoTags[0] != "bk_cmdb_saas/web:1.0" {
		t.Fatalf("标签丢失: %v", got.RepoTags)
	}
}

func TestTarballRoundTrip(t *testing.T) {
	src := t.TempDir()
	img := sampleImage(t, src)
	dest := filepath.Join(src, "image.tar")
	if err := img.Write(FormatDockerArchive, dest); err != nil {
		t.Fatal(err)
	}

	got, format, err := Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatDockerArchive {
		t.Fatalf("格式探测错误: %s", format)
	}
	assertSameConfig(t, img.V1, got.V1)
	if len(got.RepoTags) == 0 || got.RepoTags[0] != "bk_cmdb_saas/web:1.0" {
		t.Fatalf("标签丢失: %v", got.RepoTags)
	}
}

// assertSameConfig 断言 config 原始字节一致（created / history 不丢）。
func assertSameConfig(t *testing.T, want, got v1.Image) {
	t.Helper()
	wantRaw, err := want.RawConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	gotRaw, err := got.RawConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wantRaw, gotRaw) {
		t.Fatalf("config 字节不一致:\nwant %s\ngot  %s", wantRaw, gotRaw)
	}
}
