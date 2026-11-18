package lifecycle

import (
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"learn-cnb-paketo/tools/cnb/internal/spec/imagefmt"
)

func imageWithEnv(t *testing.T, env []string) *imagefmt.Image {
	t.Helper()
	img, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{
		Architecture: "amd64",
		OS:           "linux",
		Config:       v1.Config{Env: env},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &imagefmt.Image{V1: img}
}

func TestEnsureStandardPath(t *testing.T) {
	img := imageWithEnv(t, []string{"HOME=/app", "PATH=/cnb/process:/cnb/lifecycle:"})
	if err := ensureStandardPath(img); err != nil {
		t.Fatal(err)
	}
	cf, err := img.V1.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	want := "PATH=/cnb/process:/cnb/lifecycle:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	if cf.Config.Env[1] != want {
		t.Fatalf("PATH 未补齐:\n got: %s\nwant: %s", cf.Config.Env[1], want)
	}
}

func TestEnsureStandardPathIdempotent(t *testing.T) {
	img := imageWithEnv(t, []string{"PATH=/cnb/process:/cnb/lifecycle:"})
	if err := ensureStandardPath(img); err != nil {
		t.Fatal(err)
	}
	bytes1 := imageManifestString(t, img)
	if err := ensureStandardPath(img); err != nil {
		t.Fatal(err)
	}
	if got := imageManifestString(t, img); got != bytes1 {
		t.Fatal("二次调用不应再变化")
	}
}

// imageManifestString 用 config 序列化结果判断镜像是否变化。
func imageManifestString(t *testing.T, img *imagefmt.Image) string {
	t.Helper()
	cf, err := img.V1.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(cf.Config.Env, "\n")
}
