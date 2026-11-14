package checks

import "encoding/json"

// Metadata 是 builder 镜像的 io.buildpacks.builder.metadata。
// 字段名由该规范决定，不适用仓库的 snake_case 约定。
type Metadata struct {
	Description string      `json:"description"`
	Stack       Stack       `json:"stack"`
	Buildpacks  []Buildpack `json:"buildpacks"`
	Lifecycle   Lifecycle   `json:"lifecycle"`
}

// Stack 描述 build/run 镜像对。
type Stack struct {
	RunImage RunImage `json:"runImage"`
}

// RunImage 描述 run 镜像引用。
type RunImage struct {
	Image string `json:"image"`
}

// Buildpack 是 metadata 中的 buildpack 条目。
type Buildpack struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	API      string `json:"api"`
	Homepage string `json:"homepage,omitempty"`
}

// Lifecycle 是 metadata 中的 lifecycle 信息。
type Lifecycle struct {
	Version string `json:"version"`
	API     API    `json:"api"`
}

// API 描述 buildpack 与 platform 的 API 版本。
type API struct {
	Buildpack string `json:"buildpack"`
	Platform  string `json:"platform"`
}

// JSON 输出紧凑的 metadata，供 builder 打标签使用。
func (m *Metadata) JSON() (string, error) {
	b, err := json.Marshal(m)
	return string(b), err
}
