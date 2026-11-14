package builder

import "bksmartpacker/pkg/config"

// Buildpack ...
type Buildpack struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"`
}

// GetDefaultBuildpacks 获取默认的 buildpack 配置
func GetDefaultBuildpacks() map[string]Buildpack {
	return map[string]Buildpack{
		"Python": {Name: "bk-buildpack-python", Version: config.G.Bp.Python.Version, Type: config.G.Bp.Python.Type},
		"NodeJS": {Name: "bk-buildpack-nodejs", Version: config.G.Bp.NodeJS.Version, Type: config.G.Bp.NodeJS.Type},
		"Go":     {Name: "bk-buildpack-go", Version: config.G.Bp.Go.Version, Type: config.G.Bp.Go.Type},
		"Apt":    {Name: "bk-buildpack-apt", Version: config.G.Bp.Apt.Version, Type: config.G.Bp.Apt.Type},
	}
}

// GetDefaultVersionByBPName 根据 buildpack 的名字, 获取它的默认版本
func GetDefaultVersionByBPName(name string) string {
	// python 的 buildpack 配置 https://github.com/TencentBlueKing/blueking-paas/blob/builder-stack/cloudnative-buildpacks/buildpacks/bk-buildpack-python/cnb-buildpack/buildpack.toml
	for _, v := range GetDefaultBuildpacks() {
		if v.Name == name {
			return v.Version
		}
	}
	panic("unknown buildpack name: " + name)
}

// GetDefaultTypeByBPName 根据 buildpack 的名字, 获取它的默认类型
func GetDefaultTypeByBPName(name string) string {
	for _, v := range GetDefaultBuildpacks() {
		if v.Name == name {
			return v.Type
		}
	}
	panic("unknown buildpack name: " + name)
}
