package checks

// Project-wide conventions and file names.
const (
	// BuildpackAPI is the CNB Buildpack API used by this repository.
	BuildpackAPI = "0.12"
	// PlatformAPI is the supported CNB Platform API.
	PlatformAPI = "0.15"

	// FileBuildpackManifest is the buildpack metadata file name.
	FileBuildpackManifest = "buildpack.toml"
	// FileOfficialMetadata is the metadata file saved when extracting an official builder.
	FileOfficialMetadata = "builder-metadata.json"

	// StageRun is the run image stage name.
	StageRun = "run"

	// AssetLifecyclePrefix is the lifecycle asset reference prefix.
	AssetLifecyclePrefix = "lifecycle@"
)
