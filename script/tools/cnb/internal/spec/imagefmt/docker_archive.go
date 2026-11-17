package imagefmt

import (
	"encoding/json"
	"os"
	"path/filepath"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// dockerManifestEntry 是 docker-archive 的 manifest.json 元素。
type dockerManifestEntry struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

type dockerArchive struct{}

func (dockerArchive) Name() string { return FormatDockerArchive }

// Detect 判断目录或 tar 是否为 docker-archive。
func (dockerArchive) Detect(path string) bool {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return exists(join(path, FileManifestJSON)) && !exists(join(path, FileOCILayout))
	}
	return false
}

// Read 解析 docker-archive 目录。
func (dockerArchive) Read(path string) (*Image, error) {
	data, err := os.ReadFile(join(path, FileManifestJSON))
	if err != nil {
		return nil, errs.Wrap("read "+FileManifestJSON, err)
	}
	var entries []dockerManifestEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, errs.Wrap("parse "+FileManifestJSON, err)
	}
	if len(entries) == 0 {
		return nil, &errs.Invalid{Field: FileManifestJSON, Value: path, Reason: "empty manifest"}
	}
	e := entries[0]
	img := &Image{RepoTags: e.RepoTags}
	if err := readImageConfig(join(path, e.Config), img); err != nil {
		return nil, err
	}
	img.ConfigRaw, _ = os.ReadFile(join(path, e.Config))
	img.ConfigDigest = digestOfFile(join(path, e.Config))
	for _, l := range e.Layers {
		img.Layers = append(img.Layers, Layer{
			MediaType: MediaTypeOCILayer,
			Path:      join(path, l),
			Size:      fileSize(join(path, l)),
		})
	}
	return img, nil
}

// Write 写出 docker-archive 目录。
func (dockerArchive) Write(img *Image, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	cfgName, err := writeImageConfig(img, dest)
	if err != nil {
		return err
	}
	entries := make([]dockerManifestEntry, 0, 1)
	e := dockerManifestEntry{Config: cfgName, RepoTags: img.RepoTags}
	for i, l := range img.Layers {
		dir := img.Layers[i].Digest
		if dir == "" {
			dir = digestOf(i)
		}
		if err := copyLayer(l, join(dest, dir)); err != nil {
			return err
		}
		e.Layers = append(e.Layers, filepath.Join(dir, LayerTarName))
	}
	entries = append(entries, e)
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return os.WriteFile(join(dest, FileManifestJSON), data, 0o644)
}
