package imagefmt

import (
	"path/filepath"
	"strings"
)

// descriptor 是 docker 兼容 manifest.json 里的层描述。
type descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

// compatManifest 是 docker-archive manifest.json 的元素（含 LayerSources）。
type compatManifest struct {
	Config       string                `json:"Config"`
	RepoTags     []string              `json:"RepoTags"`
	Layers       []string              `json:"Layers"`
	LayerSources map[string]descriptor `json:"LayerSources"`
}

// writeCompatFiles 在 OCI layout 目录内补 docker 兼容文件：
// manifest.json（含 LayerSources）与 repositories。
func writeCompatFiles(dir string, img *Image) error {
	cfgName, err := img.V1.ConfigName()
	if err != nil {
		return err
	}
	entry := compatManifest{
		Config:       blobPath(cfgName.String()),
		RepoTags:     img.RepoTags,
		LayerSources: map[string]descriptor{},
	}
	layers, err := img.V1.Layers()
	if err != nil {
		return err
	}
	var last string
	for _, l := range layers {
		d, err := l.Digest()
		if err != nil {
			return err
		}
		size, err := l.Size()
		if err != nil {
			return err
		}
		mt, err := l.MediaType()
		if err != nil {
			return err
		}
		path := blobPath(d.String())
		entry.Layers = append(entry.Layers, path)
		entry.LayerSources[d.String()] = descriptor{MediaType: string(mt), Digest: d.String(), Size: size}
		last = strings.TrimPrefix(d.String(), "sha256:")
	}
	if err := writeJSON(filepath.Join(dir, "manifest.json"), []compatManifest{entry}); err != nil {
		return err
	}
	if len(entry.Layers) == 0 || len(img.RepoTags) == 0 {
		return nil
	}
	repo, tag := splitRepoTag(img.RepoTags[0])
	return writeJSON(filepath.Join(dir, "repositories"), map[string]map[string]string{repo: {tag: last}})
}

// blobPath 返回 digest 对应的 blob 相对路径。
func blobPath(digest string) string {
	return filepath.Join("blobs", "sha256", strings.TrimPrefix(digest, "sha256:"))
}

// splitRepoTag 拆分 repo 与 tag（无 tag 视为 latest），并去掉 docker.io 前缀。
func splitRepoTag(repoTag string) (repo, tag string) {
	repo, tag = repoTag, "latest"
	if i := strings.LastIndex(repoTag, ":"); i > strings.LastIndex(repoTag, "/") {
		repo, tag = repoTag[:i], repoTag[i+1:]
	}
	return strings.TrimPrefix(repo, "docker.io/"), tag
}

// manifestAnnotations 生成 OCI index 的 containerd 注解。
func manifestAnnotations(repoTag string) map[string]string {
	repo, tag := splitRepoTag(repoTag)
	first := strings.SplitN(repo, "/", 2)[0]
	if !strings.Contains(first, ".") && !strings.Contains(first, ":") {
		repo = "docker.io/" + repo
	}
	return map[string]string{
		"io.containerd.image.name":          repo + ":" + tag,
		"org.opencontainers.image.ref.name": tag,
	}
}
