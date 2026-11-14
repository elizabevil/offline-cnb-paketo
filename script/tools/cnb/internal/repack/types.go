package repack

// 载荷结构：键名由 OCI image-spec / docker-archive 固定，不是仓库 snake_case 约定。

type blobDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type ociManifest struct {
	SchemaVersion int              `json:"schemaVersion"`
	MediaType     string           `json:"mediaType"`
	Config        blobDescriptor   `json:"config"`
	Layers        []blobDescriptor `json:"layers"`
}

type indexAnnotations struct {
	ContainerdName string `json:"io.containerd.image.name"`
	RefName        string `json:"org.opencontainers.image.ref.name"`
}

type indexEntry struct {
	MediaType   string           `json:"mediaType"`
	Digest      string           `json:"digest"`
	Size        int64            `json:"size"`
	Annotations indexAnnotations `json:"annotations"`
}

type ociIndex struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Manifests     []indexEntry `json:"manifests"`
}

type ociLayout struct {
	ImageLayoutVersion string `json:"imageLayoutVersion"`
}

type legacyManifestEntry struct {
	Config       string                    `json:"Config"`
	RepoTags     []string                  `json:"RepoTags"`
	Layers       []string                  `json:"Layers"`
	LayerSources map[string]blobDescriptor `json:"LayerSources"`
}

type dockerManifestEntry struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}
