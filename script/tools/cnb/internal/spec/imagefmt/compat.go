package imagefmt

import (
	"strings"
)

// compatLayerSources 是 docker-archive 的 LayerSources 映射（digest → 描述）。
type compatManifest struct {
	Config       string                   `json:"Config"`
	RepoTags     []string                 `json:"RepoTags"`
	Layers       []string                 `json:"Layers"`
	LayerSources map[string]ociDescriptor `json:"LayerSources"`
}

// writeCompatFiles 在 OCI layout 内附 docker 兼容文件：
// manifest.json（含 LayerSources）与 repositories，便于消费方按 docker-archive 习惯读取。
func writeCompatFiles(dest string, img *Image, man ociManifest) error {
	configPath := blobPath(man.Config.Digest)
	entry := compatManifest{Config: configPath, RepoTags: img.RepoTags, LayerSources: map[string]ociDescriptor{}}
	for _, l := range man.Layers {
		entry.Layers = append(entry.Layers, blobPath(l.Digest))
		entry.LayerSources[l.Digest] = l
	}
	if err := writeJSON(join(dest, FileManifestJSON), []compatManifest{entry}); err != nil {
		return err
	}
	if len(man.Layers) == 0 || len(img.RepoTags) == 0 {
		return nil
	}
	repo, tag := splitRepoTag(img.RepoTags[0])
	last := strings.TrimPrefix(man.Layers[len(man.Layers)-1].Digest, DigestPrefix)
	return writeJSON(join(dest, "repositories"), map[string]map[string]string{repo: {tag: last}})
}

// splitRepoTag 拆分 repo 与 tag，并去掉 docker.io 前缀。
func splitRepoTag(repoTag string) (repo, tag string) {
	if i := strings.LastIndex(repoTag, ":"); i > strings.LastIndex(repoTag, "/") {
		repo, tag = repoTag[:i], repoTag[i+1:]
	} else {
		repo, tag = repoTag, "latest"
	}
	return strings.TrimPrefix(repo, "docker.io/"), tag
}

// readCompatRepoTags 从 docker 兼容的 manifest.json 读取镜像标签（可能不存在）。
func readCompatRepoTags(dir string) []string {
	var entries []compatManifest
	if err := readJSON(join(dir, FileManifestJSON), &entries); err != nil {
		return nil
	}
	if len(entries) == 0 {
		return nil
	}
	return entries[0].RepoTags
}

// withRegistry 给无 registry 的 repo 补默认 docker.io 前缀。
func withRegistry(repo, tag string) string {
	name := repo
	if first := strings.SplitN(repo, "/", 2)[0]; !strings.Contains(first, ".") && !strings.Contains(first, ":") {
		name = "docker.io/" + repo
	}
	if tag == "" {
		return name
	}
	return name + ":" + tag
}
