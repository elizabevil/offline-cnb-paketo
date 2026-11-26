package types

// ArtifactJSONVersion 是 artifact.json 的协议版本。
const ArtifactJSONVersion = "1.0"

// ArtifactJSON 是 v2 s-mart 的构建映射文件。
type ArtifactJSON struct {
	Version      string                    `json:"version"`
	Runtime      ArtifactRuntime           `json:"runtime"`
	AppArtifacts map[string]ArtifactModule `json:"app_artifacts"`
}

// ArtifactRuntime 描述应用的运行时基础镜像和架构。
type ArtifactRuntime struct {
	BaseImageID  string `json:"base_image_id"`
	Architecture string `json:"architecture"`
}

// ArtifactModule 描述一个模块使用的镜像包与进程入口。
type ArtifactModule struct {
	ImageTar        string              `json:"image_tar"`
	ProcEntrypoints map[string][]string `json:"proc_entrypoints"`
}
