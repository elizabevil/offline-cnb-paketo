package imagefmt

import (
	"encoding/json"
	"testing"
)

func TestEnsureStandardPath(t *testing.T) {
	img := &Image{ConfigRaw: []byte(`{"architecture":"amd64","os":"linux","config":{"Env":["HOME=/app","PATH=/cnb/process:/cnb/lifecycle:"]},"rootfs":{"type":"layers","diff_ids":[]}}`)}
	if err := img.EnsureStandardPath(); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Config struct{ Env []string } `json:"config"`
	}
	if err := json.Unmarshal(img.ConfigRaw, &got); err != nil {
		t.Fatal(err)
	}
	want := "PATH=/cnb/process:/cnb/lifecycle:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	if got.Config.Env[1] != want {
		t.Fatalf("PATH 未补齐:\n got: %s\nwant: %s", got.Config.Env[1], want)
	}
}

func TestEnsureStandardPathIdempotent(t *testing.T) {
	img := &Image{ConfigRaw: []byte(`{"config":{"Env":["PATH=/cnb/process:/cnb/lifecycle:"]}}`)}
	_ = img.EnsureStandardPath()
	first := string(img.ConfigRaw)
	_ = img.EnsureStandardPath()
	if string(img.ConfigRaw) != first {
		t.Fatal("二次调用不应再变化")
	}
}
