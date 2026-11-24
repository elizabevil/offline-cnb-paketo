package builder

// 本地 order 只包含以下 Paketo 组件；不再兼容 BK 自带 buildpack。
const (
	BuildpackGo     = "paketo-buildpacks/go"
	BuildpackNodeJS = "paketo-buildpacks/nodejs"
	BuildpackPython = "paketo-buildpacks/python"
)

// Buildpack 描述一个 buildpack 条目。
type Buildpack struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Type    string `json:"type,omitempty"`
}

// GetDefaultBuildpacks 返回语言到本地 order 的映射。
func GetDefaultBuildpacks() map[string]Buildpack {
	return map[string]Buildpack{
		"Go":     {Name: BuildpackGo},
		"NodeJS": {Name: BuildpackNodeJS},
		"Python": {Name: BuildpackPython},
	}
}
