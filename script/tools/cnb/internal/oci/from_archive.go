package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// 媒体类型（由 OCI 规范决定）。
const (
	MediaTypeManifest = "application/vnd.oci.image.manifest.v1+json"
	MediaTypeConfig   = "application/vnd.oci.image.config.v1+json"
	MediaTypeLayer    = "application/vnd.oci.image.layer.v1.tar+gzip"
	MediaTypeIndex    = "application/vnd.oci.image.index.v1+json"
)

// ToLayout 把 docker-archive 转成 OCI layout（ToDockerArchive 的逆操作）。
func ToLayout(archiveTar, layoutDir string) error {
	f, err := os.Open(archiveTar)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// 先把归档读进内存索引：manifest.json 可能出现在任意位置
	files := map[string][]byte{}
	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return err
		}
		files[filepath.Clean(h.Name)] = data
	}

	var manifests []dockerManifest
	if err := json.Unmarshal(files["manifest.json"], &manifests); err != nil {
		return fmt.Errorf("解析 manifest.json: %w", err)
	}
	if len(manifests) == 0 {
		return fmt.Errorf("manifest.json 为空")
	}
	return writeLayout(layoutDir, manifests[0], files)
}

// writeLayout 生成 blobs/index.json/oci-layout。
func writeLayout(dir string, m dockerManifest, files map[string][]byte) error {
	blobDir := filepath.Join(dir, "blobs", "sha256")
	if err := os.MkdirAll(blobDir, 0o755); err != nil {
		return err
	}

	cfg, ok := files[m.Config]
	if !ok {
		return fmt.Errorf("缺少配置 %s", m.Config)
	}
	cfgDigest, err := writeBlob(blobDir, cfg)
	if err != nil {
		return err
	}

	var layers []descriptor
	for _, name := range m.Layers {
		raw, ok := files[name]
		if !ok {
			return fmt.Errorf("缺少层 %s", name)
		}
		gz, err := gzipBytes(raw)
		if err != nil {
			return err
		}
		digest, err := writeBlob(blobDir, gz)
		if err != nil {
			return err
		}
		layers = append(layers, descriptor{MediaType: MediaTypeLayer, Digest: digest, Size: int64(len(gz))})
	}

	man := manifest{
		Config: descriptor{MediaType: MediaTypeConfig, Digest: cfgDigest, Size: int64(len(cfg))},
		Layers: layers,
	}
	manData, err := json.Marshal(man)
	if err != nil {
		return err
	}
	manDigest, err := writeBlob(blobDir, manData)
	if err != nil {
		return err
	}

	idx := index{Manifests: []descriptor{{
		MediaType: MediaTypeManifest, Digest: manDigest, Size: int64(len(manData)),
	}}}
	idxData, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), idxData, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "oci-layout"), []byte(`{"imageLayoutVersion":"1.0.0"}`), 0o644)
}

// writeBlob 写入 blob 并返回 digest。
func writeBlob(dir string, data []byte) (string, error) {
	sum := sha256.Sum256(data)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(dir, hex.EncodeToString(sum[:])), data, 0o644); err != nil {
		return "", err
	}
	return digest, nil
}

// gzipBytes 返回 gzip 压缩后的字节。
func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
