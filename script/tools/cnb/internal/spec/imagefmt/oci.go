package imagefmt

import (
	"os"
	"path/filepath"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// ociIndex / ociManifest / ociDescriptor 是 OCI layout 的最小结构。
type ociIndex struct {
	SchemaVersion int             `json:"schemaVersion"`
	MediaType     string          `json:"mediaType"`
	Manifests     []ociDescriptor `json:"manifests"`
}

type ociManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	MediaType     string          `json:"mediaType"`
	Config        ociDescriptor   `json:"config"`
	Layers        []ociDescriptor `json:"layers"`
}

type ociDescriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ociLayout struct{}

func (ociLayout) Name() string { return FormatOCILayout }

// Detect 判断目录是否为 OCI layout。
func (ociLayout) Detect(path string) bool {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return exists(join(path, FileOCILayout)) && exists(join(path, FileIndexJSON))
	}
	return false
}

// Read 解析 OCI layout 目录。
func (ociLayout) Read(path string) (*Image, error) {
	var idx ociIndex
	if err := readJSON(join(path, FileIndexJSON), &idx); err != nil {
		return nil, err
	}
	if len(idx.Manifests) == 0 {
		return nil, &errs.Invalid{Field: FileIndexJSON, Value: path, Reason: "empty index"}
	}
	var man ociManifest
	if err := readJSON(join(path, blobPath(idx.Manifests[0].Digest)), &man); err != nil {
		return nil, err
	}
	img := &Image{Config: Config{}, ConfigDigest: man.Config.Digest}
	cfgPath := join(path, blobPath(man.Config.Digest))
	if data, err := os.ReadFile(cfgPath); err == nil {
		img.ConfigRaw = data
	}
	if err := readImageConfig(cfgPath, img); err != nil {
		return nil, err
	}
	for _, l := range man.Layers {
		p := join(path, blobPath(l.Digest))
		img.Layers = append(img.Layers, Layer{MediaType: l.MediaType, Digest: l.Digest, Size: l.Size, Path: p})
	}
	img.RepoTags = readCompatRepoTags(path)
	return img, nil
}

// Write 写出 OCI layout 目录。
func (ociLayout) Write(img *Image, dest string) error {
	blobs := join(dest, DirBlobs, DirSHA256)
	if err := os.MkdirAll(blobs, 0o755); err != nil {
		return err
	}
	cfgData, err := configBytes(img)
	if err != nil {
		return err
	}
	cfgDigest, cfgSize, err := writeBlobData(blobs, cfgData)
	if err != nil {
		return err
	}
	man := ociManifest{
		SchemaVersion: 2, MediaType: MediaTypeOCIManifest,
		Config: ociDescriptor{MediaType: MediaTypeOCIConfig, Digest: cfgDigest, Size: cfgSize},
	}
	for i, l := range img.Layers {
		digest, size, err := writeLayerBlob(blobs, l, i)
		if err != nil {
			return err
		}
		man.Layers = append(man.Layers, ociDescriptor{MediaType: MediaTypeOCILayer, Digest: digest, Size: size})
	}
	manDigest, manSize, err := writeBlobJSON(blobs, man)
	if err != nil {
		return err
	}
	entry := ociDescriptor{MediaType: MediaTypeOCIManifest, Digest: manDigest, Size: manSize}
	if len(img.RepoTags) > 0 {
		repo, tag := splitRepoTag(img.RepoTags[0])
		entry.Annotations = map[string]string{
			"io.containerd.image.name":          withRegistry(repo, tag),
			"org.opencontainers.image.ref.name": tag,
		}
	}
	idx := ociIndex{SchemaVersion: 2, MediaType: MediaTypeOCIIndex, Manifests: []ociDescriptor{entry}}
	if err := writeJSON(join(dest, FileIndexJSON), idx); err != nil {
		return err
	}
	if err := writeJSON(join(dest, FileOCILayout), map[string]string{"imageLayoutVersion": "1.0.0"}); err != nil {
		return err
	}
	return writeCompatFiles(dest, img, man)
}

// blobPath 返回 digest 对应的 blob 路径。
func blobPath(digest string) string {
	name := digest
	if len(digest) > len(DigestPrefix) && digest[:len(DigestPrefix)] == DigestPrefix {
		name = digest[len(DigestPrefix):]
	}
	return filepath.Join(DirBlobs, DirSHA256, name)
}
