package imagefmt

import (
	"os"
	"path/filepath"
	"testing"
)

// makeLayer 生成一个最小层 tar 文件。
func makeLayer(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("layer-content-"+name), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func sampleImage(t *testing.T, dir string) *Image {
	return &Image{
		RepoTags: []string{"demo/app:1.0"},
		OS:       "linux",
		Arch:     "amd64",
		Config: Config{
			Entrypoint: []string{"/cnb/process/web"},
			Env:        []string{"APP=1"},
			WorkingDir: "/app",
			DiffIDs:    []string{"sha256:aaa"},
		},
		Layers: []Layer{{MediaType: MediaTypeOCILayer, Path: makeLayer(t, dir, "l0.tar")}},
	}
}

func TestDockerArchiveRoundTrip(t *testing.T) {
	src := t.TempDir()
	img := sampleImage(t, src)
	dst := filepath.Join(src, "docker")
	if err := Write(img, FormatDockerArchive, dst); err != nil {
		t.Fatal(err)
	}
	f, err := Detect(dst)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name() != FormatDockerArchive {
		t.Fatalf("格式探测错误: %s", f.Name())
	}
	got, err := f.Read(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got.Arch != img.Arch || got.Config.Entrypoint[0] != img.Config.Entrypoint[0] {
		t.Fatalf("config 不一致: %+v", got)
	}
	if len(got.Layers) != 1 {
		t.Fatalf("层数错误: %d", len(got.Layers))
	}
}

func TestOCIRoundTrip(t *testing.T) {
	src := t.TempDir()
	img := sampleImage(t, src)
	dst := filepath.Join(src, "oci")
	if err := Write(img, FormatOCILayout, dst); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dst, FileOCILayout)) || !exists(filepath.Join(dst, FileIndexJSON)) {
		t.Fatal("缺少 OCI layout 标记文件")
	}
	got, format, err := Read(dst)
	if err != nil {
		t.Fatal(err)
	}
	if format.Name() != FormatOCILayout {
		t.Fatalf("格式探测错误: %s", format.Name())
	}
	if got.Config.WorkingDir != "/app" || len(got.Layers) != 1 {
		t.Fatalf("读取结果不符: %+v", got)
	}
}

func TestDetectUnknown(t *testing.T) {
	if _, err := Detect(t.TempDir()); err == nil {
		t.Fatal("空目录不应被识别为镜像格式")
	}
	if err := Write(&Image{}, "bogus", t.TempDir()); err == nil {
		t.Fatal("未知格式应报错")
	}
}
