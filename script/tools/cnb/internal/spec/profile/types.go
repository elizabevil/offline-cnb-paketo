// Package profile 定义流水线配置（K8s 结构 + 统一 exec stage）的解析与校验。
package profile

// APIVersion 与 Kind 的合法取值。
const (
	// APIVersionV1 是当前唯一支持的版本。
	APIVersionV1 = "cnb/v1"
	// KindPipeline 是流水线资源类型。
	KindPipeline = "Pipeline"
)

// ErrFieldVersion 等是校验错误里的字段名。
const (
	FieldAPIVersion = "apiVersion"
	FieldKind       = "kind"
	FieldMetadata   = "metadata.name"
	FieldProject    = "spec.project"
	FieldStages     = "spec.stages"
)

// File 是流水线配置的根结构。
type File struct {
	APIVersion string   `toml:"apiVersion"`
	Kind       string   `toml:"kind"`
	Metadata   Metadata `toml:"metadata"`
	Spec       Spec     `toml:"spec"`
}

// Metadata 是资源元信息。
type Metadata struct {
	Name string `toml:"name"`
}

// Spec 是流水线主体。
type Spec struct {
	Project Project          `toml:"project"`
	Assets  map[string]Asset `toml:"assets"`
	Stages  []Stage          `toml:"stages"`
}

// Project 是项目标识与默认平台。
type Project struct {
	Dir      string `toml:"dir"`
	Platform string `toml:"platform"`
}

// Asset 是本项目锁定的物料版本。
type Asset struct {
	Version   string   `toml:"version,omitempty"`
	Platforms []string `toml:"platforms,omitempty"`
}

// Stage 是一个执行阶段：一条命令 + 运行位置。
type Stage struct {
	Name            string            `toml:"name"`
	Run             []string          `toml:"run"`
	Workdir         string            `toml:"workdir"`
	Env             map[string]string `toml:"env,omitempty"`
	Timeout         string            `toml:"timeout,omitempty"`
	ContinueOnError bool              `toml:"continueOnError,omitempty"`
}
