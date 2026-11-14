package checks

// 规范与文件约定。
const (
	// BuildpackAPI 是本项目 buildpack 使用的 CNB Buildpack API。
	BuildpackAPI = "0.12"
	// PlatformAPI 是本项目支持的 CNB Platform API。
	PlatformAPI = "0.15"

	// FileBuildpackManifest 是 buildpack 元数据文件名。
	FileBuildpackManifest = "buildpack.toml"
	// FileOfficialMetadata 是从官方 builder 提取时保存的 metadata 文件名。
	FileOfficialMetadata = "builder-metadata.json"

	// StageRun 是 run 镜像阶段名。
	StageRun = "run"

	// AssetLifecyclePrefix 是 lifecycle 物料引用前缀。
	AssetLifecyclePrefix = "lifecycle@"

	// stackLabelPattern 匹配 Dockerfile 中的 stack id 标签。
	stackLabelPattern = `io\.buildpacks\.stack\.id="([^"]+)"`
)
