// Package oci 在 OCI layout 与 docker-archive 之间转换，避免依赖容器 daemon。
package oci

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
)

type index struct {
	Manifests []descriptor `json:"manifests"`
}

type descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type manifest struct {
	Config descriptor   `json:"config"`
	Layers []descriptor `json:"layers"`
}

type dockerManifest struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

func blobPath(root, digest string) string {
	return filepath.Join(root, "blobs", strings.Replace(digest, ":", "/", 1))
}

// ToDockerArchive 把 OCI layout 转成 docker-archive tar。
// repoTag 形如 "repo/name:tag"，写入 manifest.json 的 RepoTags。
func ToDockerArchive(layoutDir, repoTag, outTar string) error {
	idxData, err := os.ReadFile(filepath.Join(layoutDir, "index.json"))
	if err != nil {
		return errs.Wrap("读取 index.json", err)
	}
	var idx index
	if err := json.Unmarshal(idxData, &idx); err != nil {
		return err
	}
	if len(idx.Manifests) == 0 {
		return fmt.Errorf("index.json 没有 manifests")
	}
	manData, err := os.ReadFile(blobPath(layoutDir, idx.Manifests[0].Digest))
	if err != nil {
		return err
	}
	var man manifest
	if err := json.Unmarshal(manData, &man); err != nil {
		return err
	}

	cfgName := strings.TrimPrefix(man.Config.Digest, "sha256:") + ".json"

	out, err := os.Create(outTar)
	if err != nil {
		return err
	}
	defer out.Close()
	tw := tar.NewWriter(out)
	defer tw.Close()

	cfgData, err := os.ReadFile(blobPath(layoutDir, man.Config.Digest))
	if err != nil {
		return err
	}
	if err := writeFile(tw, cfgName, cfgData); err != nil {
		return err
	}

	var layers []string
	for _, l := range man.Layers {
		id := strings.TrimPrefix(l.Digest, "sha256:")
		layers = append(layers, id+"/layer.tar")
		if err := writeLayer(tw, blobPath(layoutDir, l.Digest), id+"/layer.tar"); err != nil {
			return err
		}
	}

	dm := []dockerManifest{{Config: cfgName, RepoTags: []string{repoTag}, Layers: layers}}
	dmData, err := json.Marshal(dm)
	if err != nil {
		return err
	}
	return writeFile(tw, "manifest.json", dmData)
}

func writeFile(tw *tar.Writer, name string, data []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: fsutil.FileMode, Size: int64(len(data))}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

// writeLayer 把（可能 gzip 的）blob 解成未压缩 layer.tar 写入归档。
func writeLayer(tw *tar.Writer, src, name string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	var r io.Reader = f
	if b, err := readMagic(f); err != nil {
		return err
	} else if b {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		r = gz
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return writeFile(tw, name, data)
}

// readMagic 判断文件是否为 gzip。
func readMagic(f *os.File) (bool, error) {
	buf := make([]byte, 2)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}
	return n == 2 && buf[0] == 0x1f && buf[1] == 0x8b, nil
}
