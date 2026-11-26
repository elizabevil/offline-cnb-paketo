package builder

const (
	// BuildpackTypeOCIEmbedded 表示 buildpack 已嵌入 builder 镜像。
	BuildpackTypeOCIEmbedded = "oci-embedded"
	// PackagingVersionV1 是旧的每模块 tgz 方案。
	PackagingVersionV1 = "v1"
	// PackagingVersionV2 是构建组 + artifact.json 方案。
	PackagingVersionV2 = "v2"
	// DefaultBaseImageID 是平台未指定时的运行时基础镜像 ID。
	DefaultBaseImageID = "default"
	// ImageTarExt 是 v2 构建组镜像包的扩展名。
	ImageTarExt = ".tar"
)
