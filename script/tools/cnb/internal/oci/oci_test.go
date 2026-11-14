package oci

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestToDockerArchive(t *testing.T) {
	dir := t.TempDir()
	// 造一个最小 OCI layout：config + 一层（gzip 的 tar）
	cfg := []byte(`{"architecture":"amd64","os":"linux"}`)
	write := func(digest string, data []byte) string {
		name := "sha256:" + digest
		p := filepath.Join(dir, "blobs", "sha256", digest)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, data, 0o644)
		return name
	}
	cfgDigest := "1111111111111111111111111111111111111111111111111111111111111111"
	write(cfgDigest, cfg)

	// 层：gzip(tar{file})
	var buf []byte
	{
		gz, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed)
		_ = gz
	}
	// 简化：直接把未压缩 tar 当层内容（readMagic 会判定非 gzip）
	var lb []byte
	{
		f, _ := os.CreateTemp(t.TempDir(), "l")
		tw := tar.NewWriter(f)
		tw.WriteHeader(&tar.Header{Name: "a", Mode: 0o644, Size: 1})
		tw.Write([]byte("x"))
		tw.Close()
		f.Close()
		lb, _ = os.ReadFile(f.Name())
		_ = buf
	}
	layerDigest := "2222222222222222222222222222222222222222222222222222222222222222"
	write(layerDigest, lb)

	man := map[string]any{
		"config": map[string]any{"mediaType": "application/vnd.oci.image.config.v1+json", "digest": "sha256:" + cfgDigest, "size": len(cfg)},
		"layers": []any{map[string]any{"mediaType": "application/vnd.oci.image.layer.v1.tar", "digest": "sha256:" + layerDigest, "size": len(lb)}},
	}
	mb, _ := json.Marshal(man)
	manDigest := "3333333333333333333333333333333333333333333333333333333333333333"
	write(manDigest, mb)
	idx := map[string]any{"manifests": []any{map[string]any{"mediaType": "application/vnd.oci.image.manifest.v1+json", "digest": "sha256:" + manDigest, "size": len(mb)}}}
	ib, _ := json.Marshal(idx)
	os.WriteFile(filepath.Join(dir, "index.json"), ib, 0o644)

	out := filepath.Join(t.TempDir(), "out.tar")
	if err := ToDockerArchive(dir, "demo/app:1", out); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(out)
	defer f.Close()
	tr := tar.NewReader(f)
	names := map[string]bool{}
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		names[h.Name] = true
	}
	for _, want := range []string{"manifest.json", cfgDigest + ".json", layerDigest + "/layer.tar"} {
		if !names[want] {
			t.Fatalf("缺少 %s，实际 %v", want, names)
		}
	}
}
