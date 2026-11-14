package oci

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// TopLayer 返回 OCI layout 中第一个镜像的最后一层 digest。
func TopLayer(layoutDir string) (string, error) {
	idxData, err := os.ReadFile(filepath.Join(layoutDir, "index.json"))
	if err != nil {
		return "", err
	}
	var idx index
	if err := json.Unmarshal(idxData, &idx); err != nil {
		return "", err
	}
	if len(idx.Manifests) == 0 {
		return "", errNoManifest
	}
	manData, err := os.ReadFile(blobPath(layoutDir, idx.Manifests[0].Digest))
	if err != nil {
		return "", err
	}
	var man manifest
	if err := json.Unmarshal(manData, &man); err != nil {
		return "", err
	}
	if len(man.Layers) == 0 {
		return "", errNoLayers
	}
	return man.Layers[len(man.Layers)-1].Digest, nil
}

// errNoManifest 表示 layout 缺少镜像描述。
var errNoManifest = &layoutError{"layout has no manifests"}

// errNoLayers 表示镜像没有层。
var errNoLayers = &layoutError{"image has no layers"}

type layoutError struct{ msg string }

func (e *layoutError) Error() string { return e.msg }

// ManifestDigest 返回 layout 中第一个镜像的 manifest digest。
func ManifestDigest(layoutDir string) string {
	idxData, err := os.ReadFile(filepath.Join(layoutDir, "index.json"))
	if err != nil {
		return ""
	}
	var idx index
	if err := json.Unmarshal(idxData, &idx); err != nil || len(idx.Manifests) == 0 {
		return ""
	}
	return idx.Manifests[0].Digest
}

// BlobPath 返回 digest 对应的 blob 路径（供同包测试使用）。
func BlobPath(root, digest string) string { return blobPath(root, digest) }

// LayoutImagePath 把镜像引用映射为 layout 内的目录路径。
func LayoutImagePath(root, ref string) (string, error) {
	name, tag, ok := splitRef(ref)
	if !ok {
		return "", errNoManifest
	}
	if !strings.Contains(name, "/") {
		name = "library/" + name
	}
	return filepath.Join(root, "index.docker.io", name, tag), nil
}

// splitRef 拆分 name:tag，缺省 tag 为 latest。
func splitRef(ref string) (string, string, bool) {
	if strings.Contains(ref, "@") {
		return "", "", false
	}
	idx := strings.LastIndex(ref, ":")
	if idx < 0 || strings.Contains(ref[idx:], "/") {
		return ref, "latest", true
	}
	return ref[:idx], ref[idx+1:], true
}
