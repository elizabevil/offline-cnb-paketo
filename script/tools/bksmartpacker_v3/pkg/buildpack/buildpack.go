package buildpack

import (
	"github.com/samber/lo"

	"bksmartpacker/pkg/types"
)

// 本地 order 只包含以下 Paketo 组件；不再兼容 BK 自带 buildpack。
const (
	BuildpackGo     = "paketo-buildpacks/go"
	BuildpackNodeJS = "paketo-buildpacks/nodejs"
	BuildpackPython = "paketo-buildpacks/python"
)

// TypeOCIEmbedded 表示 buildpack 已嵌入 builder 镜像。
const TypeOCIEmbedded = "oci-embedded"

// Buildpack 描述一个 buildpack 条目。
type Buildpack struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Type    string `json:"type,omitempty"`
}

// Defaults 返回语言到本地 order 的映射。
func Defaults() map[string]Buildpack {
	return map[string]Buildpack{
		"Go":     {Name: BuildpackGo},
		"NodeJS": {Name: BuildpackNodeJS},
		"Python": {Name: BuildpackPython},
	}
}

// ForModule 生成模块需要的 buildpacks。
func ForModule(moduleDesc *types.ModuleDescription) []Buildpack {
	if len(moduleDesc.Spec.Build.Buildpacks) > 0 {
		return lo.Map(moduleDesc.Spec.Build.Buildpacks, func(item types.Buildpack, _ int) Buildpack {
			return Buildpack{Name: item.Name, Version: item.Version}
		})
	}
	if bp, ok := Defaults()[moduleDesc.Language]; ok {
		return []Buildpack{bp}
	}
	return nil
}
