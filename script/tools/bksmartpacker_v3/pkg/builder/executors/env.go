package executors

import "bksmartpacker/pkg/container"

// builder/lifecycle 协议环境变量 key。
// 这些不是用户配置，不能散落在 buildArgs 中。
type envKey = container.EnvKey

const (
	envRequiredBuildpacks envKey = "REQUIRED_BUILDPACKS"
	envOutputImage        envKey = "OUTPUT_IMAGE"
	envRunImage           envKey = "CNB_RUN_IMAGE"
	envSourceGetURL       envKey = "SOURCE_GET_URL"
	envUseDockerDaemon    envKey = "USE_DOCKER_DAEMON"
	envPlatformAPI        envKey = "CNB_PLATFORM_API"
	envBuildpackAliases   envKey = "CNB_BUILDPACK_ALIASES"
	envCacheImage         envKey = "CACHE_IMAGE"
	envRegistryAuth       envKey = "CNB_REGISTRY_AUTH"
	envCacheDir           envKey = "CNB_CACHE_DIR"
	envExportMode         envKey = "CNB_EXPORT_MODE"
	envLayerArchive       envKey = "CNB_LAYER_ARCHIVE"
	envRunImageArchive    envKey = "CNB_RUN_IMAGE_ARCHIVE"
)
