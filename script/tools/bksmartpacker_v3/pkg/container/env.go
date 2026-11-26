package container

// EnvKey 是运行时配置环境变量的强类型 key。
// 所有读取和写入都使用这里的常量，避免散落字符串。
type EnvKey = string

// 环境变量 key 列表。
const (
	EnvRuntime                EnvKey = "RUNTIME"
	EnvBksRuntime             EnvKey = "BKS_RUNTIME"
	EnvRuntimeDriver          EnvKey = "CNB_RUNTIME_DRIVER"
	EnvBksRuntimeDriver       EnvKey = "BKS_RUNTIME_DRIVER"
	EnvContainerCLI           EnvKey = "BKS_CONTAINER_CLI"
	EnvDockerSock             EnvKey = "BKS_DOCKER_SOCK"
	EnvExportMode             EnvKey = "BKS_EXPORT_MODE"
	EnvPlatform               EnvKey = "BKS_PLATFORM"
	EnvNetwork                EnvKey = "BKS_NETWORK"
	EnvExtraMounts            EnvKey = "BKS_EXTRA_MOUNTS"
	EnvBksRunImageArchiveFile EnvKey = "BKS_RUN_IMAGE_ARCHIVE_FILE"
	EnvCacheDir               EnvKey = "CNB_CACHE_DIR"
	EnvBksCacheDir            EnvKey = "BKS_CACHE_DIR"
	EnvWorkspace              EnvKey = "CNB_RUNTIME_WORKSPACE"
	EnvBksWorkspace           EnvKey = "BKS_RUNTIME_WORKSPACE"
	EnvRuntimeImage           EnvKey = "CNB_RUNTIME_IMAGE"
	EnvBksRuntimeImage        EnvKey = "BKS_RUNTIME_IMAGE"
	EnvSidecarTimeout         EnvKey = "CNB_SIDECAR_TIMEOUT"
	EnvBksSidecarTimeout      EnvKey = "BKS_SIDECAR_TIMEOUT"
	EnvBuildpackAliases       EnvKey = "BKS_BUILDPACK_ALIASES"
	EnvXDGRuntimeDir          EnvKey = "XDG_RUNTIME_DIR"

	// EnvRuntimeCache 是 sidecar 内 runtime storage 的根目录。
	EnvRuntimeCache EnvKey = "CNB_RUNTIME_CACHE"
	// EnvDockerHost 是 builder 容器内访问运行时 socket 的地址。
	EnvDockerHost EnvKey = "DOCKER_HOST"

	// lifecycle / builder 协议环境变量。
	EnvRequiredBuildpacks  EnvKey = "REQUIRED_BUILDPACKS"
	EnvOutputImage         EnvKey = "OUTPUT_IMAGE"
	EnvRunImage            EnvKey = "CNB_RUN_IMAGE"
	EnvSourceGetURL        EnvKey = "SOURCE_GET_URL"
	EnvUseDockerDaemon     EnvKey = "USE_DOCKER_DAEMON"
	EnvPlatformAPI         EnvKey = "CNB_PLATFORM_API"
	EnvCNBBuildpackAliases EnvKey = "CNB_BUILDPACK_ALIASES"
	EnvCacheImage          EnvKey = "CACHE_IMAGE"
	EnvRegistryAuth        EnvKey = "CNB_REGISTRY_AUTH"
	EnvCNBExportMode       EnvKey = "CNB_EXPORT_MODE"
	EnvLayerArchive        EnvKey = "CNB_LAYER_ARCHIVE"
	EnvRunImageArchive     EnvKey = "CNB_RUN_IMAGE_ARCHIVE"
)
