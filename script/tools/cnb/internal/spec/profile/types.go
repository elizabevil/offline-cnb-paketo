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
	Checks  Checks           `toml:"checks,omitempty"`
}

// Checks declares static checks that are not expressible through stage argv.
type Checks struct {
	// Dockerfiles lists Dockerfiles whose COPY sources must exist.
	Dockerfiles []DockerfileCheck `toml:"dockerfiles,omitempty"`
}

// DockerfileCheck pins one Dockerfile and the contexts its COPY uses.
type DockerfileCheck struct {
	Path     string            `toml:"path"`
	Context  string            `toml:"context,omitempty"`  // default: project dir
	Contexts map[string]string `toml:"contexts,omitempty"` // named --from contexts
}

// Project 是项目标识与默认平台。
type Project struct {
	Directory string `toml:"directory"`
	Platform  string `toml:"platform"`
	// Base 是各平台的运行时基础镜像（如 debian 的 arch 专用 tag）。
	Base map[string]string `toml:"base,omitempty"`
}

// Asset 是本项目锁定的物料版本。
type Asset struct {
	Version   string   `toml:"version,omitempty"`
	Platforms []string `toml:"platforms,omitempty"`
}

// Stage is one execution step: exactly one command, no type dispatch.
type Stage struct {
	Name            string            `toml:"name"`
	Run             []string          `toml:"run"`
	Workdir         string            `toml:"workdir"`
	Env             map[string]string `toml:"env,omitempty"`
	Timeout         string            `toml:"timeout,omitempty"`
	ContinueOnError bool              `toml:"continueOnError,omitempty"`
}
