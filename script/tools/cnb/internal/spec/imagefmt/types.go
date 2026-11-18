// Package imagefmt 读写镜像归档格式（docker-archive / OCI layout）。
package imagefmt

// 格式名。
const (
	// FormatDockerArchive 是 docker save 产物（manifest.json + <digest>/layer.tar）。
	FormatDockerArchive = "docker-archive"
	// FormatOCILayout 是 OCI image layout（oci-layout + index.json + blobs/sha256/*）。
	FormatOCILayout = "oci-layout"
)

// 媒体类型。
const (
	MediaTypeOCIConfig   = "application/vnd.oci.image.config.v1+json"
	MediaTypeOCIManifest = "application/vnd.oci.image.manifest.v1+json"
	MediaTypeOCIIndex    = "application/vnd.oci.image.index.v1+json"
	MediaTypeOCILayer    = "application/vnd.oci.image.layer.v1.tar"
)

// 文件名与目录。
const (
	FileManifestJSON = "manifest.json"
	FileIndexJSON    = "index.json"
	FileOCILayout    = "oci-layout"
	DirBlobs         = "blobs"
	DirSHA256        = "sha256"
	DigestPrefix     = "sha256:"
	LayerTarName     = "layer.tar"
)

// Image 是格式无关的统一模型。
type Image struct {
	RepoTags []string
	OS       string
	Arch     string
	// ConfigDigest 是 config blob 的 sha256（读入时填充）。
	ConfigDigest string
	// ConfigRaw 是原始 config JSON（读入时填充）；写出时原样使用，
	// 避免丢失 created / history 等本包不解析的字段。
	ConfigRaw []byte
	Config    Config
	Layers    []Layer
}

// Config 是镜像配置（只保留我们使用/生成的字段）。
type Config struct {
	Entrypoint []string          `json:"Entrypoint,omitempty"`
	Env        []string          `json:"Env,omitempty"`
	WorkingDir string            `json:"WorkingDir,omitempty"`
	Labels     map[string]string `json:"Labels,omitempty"`
	DiffIDs    []string          `json:"DiffIDs,omitempty"`
}

// Layer 是一层。
type Layer struct {
	MediaType string
	Digest    string
	Size      int64
	// Path 是层内容在解包目录里的路径（读入时填写）。
	Path string
}
