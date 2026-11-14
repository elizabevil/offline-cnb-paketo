package repack

// SmartPackage / OCI layout 的文件名与目录约定。
const (
	// WebTarName 是 SmartPackage 内的层包文件名。
	WebTarName = "web.tgz"
	// ManifestName 是 docker-archive 的清单文件名。
	ManifestName = "manifest.json"
	// IndexName 是 OCI layout 的索引文件名。
	IndexName = "index.json"
	// LayoutVersionName 是 OCI layout 的版本标记文件。
	LayoutVersionName = "oci-layout"
	// BlobsDir 是 OCI layout 的 blob 目录。
	BlobsDir = "blobs"
	// SHA256Dir 是 blob 下的算法目录。
	SHA256Dir = "sha256"
	// DigestPrefix 是 digest 的算法前缀。
	DigestPrefix = "sha256:"

	// ArchiveSuffix 是可接受的输入归档后缀。
	ArchiveSuffix = ".tar.gz"
	// 临时目录内的子目录名。
	DirPkg     = "pkg"
	DirLegacy  = "legacy"
	DirOCI     = "oci"
	RebuiltTar = "rebuilt.tar.gz"

	// TempPkgPrefix / TempWebPrefix 是临时目录前缀。
	TempPkgPrefix = "cnb-repack-pkg-"
	TempWebPrefix = "cnb-repack-web-"
)

// OCI 媒体类型（由 image-spec 固定）。
const (
	MediaTypeOCIConfig   = "application/vnd.oci.image.config.v1+json"
	MediaTypeOCIManifest = "application/vnd.oci.image.manifest.v1+json"
	MediaTypeOCIIndex    = "application/vnd.oci.image.index.v1+json"
	MediaTypeOCILayer    = "application/vnd.oci.image.layer.v1.tar"
)
