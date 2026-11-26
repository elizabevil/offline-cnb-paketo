package plan

import (
	"fmt"
	"strings"

	"bksmartpacker/pkg/buildpack"
)

// BuildPlan 是 bksmartpacker 的构建计划。
type BuildPlan struct {
	// Dest s-mart 构建产物输出路径
	Dest string `json:"dest"`
	// Src 源码路径
	Src            string              `json:"src"`
	AppCode        string              `json:"appCode"`
	AppDescContent string              `json:"appDesc"`
	LogoFileName   string              `json:"logoFileName"`
	Steps          []*ModuleBuildStep  `json:"steps"`
	Groups         []*ModuleBuildGroup `json:"groups,omitempty"`

	// PackagingVersion 打包方案: v1 | v2
	PackagingVersion string `json:"packagingVersion"`
	// BaseImageID artifact.json runtime.base_image_id
	BaseImageID string `json:"baseImageId"`
	// Architecture 目标镜像架构
	Architecture string `json:"architecture"`
	// ModuleProcesses 模块名 -> 进程列表
	ModuleProcesses map[string][]ModuleProcess `json:"moduleProcesses,omitempty"`

	BuilderImage string `json:"builderImage"`
}

// ModuleBuildStep 是一个模块的 v1 构建单元。
type ModuleBuildStep struct {
	AppCode    string `json:"appCode"`
	ModuleName string `json:"module_name"`
	Version    string `json:"version"`
	// Src 源码目录或源码压缩包的绝对路径
	Src        string                `json:"src"`
	Procfile   string                `json:"procfile"`
	Buildpacks []buildpack.Buildpack `json:"buildpacks"`
	Envs       map[string]string     `json:"envs"`

	CacheRegistry string `json:"cacheRegistry"`
	RegistryAuth  string `json:"registryAuth"`
}

// RequiredBuildpacks 生成 lifecycle 的 REQUIRED_BUILDPACKS。
func (step ModuleBuildStep) RequiredBuildpacks() string {
	return requiredBuildpacks(step.Buildpacks)
}

// BuildEnvs 返回平台环境变量。
func (step ModuleBuildStep) BuildEnvs() []string { return buildEnvList(step.Envs) }

// OutputImage 是模块的镜像 tag。
func (step ModuleBuildStep) OutputImage() string {
	return imageTag(step.AppCode, step.ModuleName, step.Version)
}

// ModuleBuildGroup 是共享同一构建产物的模块集合。
type ModuleBuildGroup struct {
	AppCode            string                `json:"appCode"`
	ModuleName         string                `json:"moduleName"`
	ModuleNames        []string              `json:"moduleNames"`
	Version            string                `json:"version"`
	SourceDir          string                `json:"sourceDir"`
	Src                string                `json:"src"`
	Procfile           string                `json:"procfile"`
	Buildpacks         []buildpack.Buildpack `json:"buildpacks"`
	Envs               map[string]string     `json:"envs"`
	OutputImageTarName string                `json:"outputImageTarName"`

	CacheRegistry string `json:"cacheRegistry"`
	RegistryAuth  string `json:"registryAuth"`
}

// RequiredBuildpacks 生成 lifecycle 的 REQUIRED_BUILDPACKS。
func (g ModuleBuildGroup) RequiredBuildpacks() string {
	return requiredBuildpacks(g.Buildpacks)
}

// BuildEnvs 返回平台环境变量。
func (g ModuleBuildGroup) BuildEnvs() []string { return buildEnvList(g.Envs) }

// OutputImage 是构建组主模块的镜像 tag。
func (g ModuleBuildGroup) OutputImage() string {
	return imageTag(g.AppCode, g.ModuleName, g.Version)
}

// ModuleProcess 是模块内的一个进程。
type ModuleProcess struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

func requiredBuildpacks(buildpacks []buildpack.Buildpack) string {
	parts := make([]string, 0, len(buildpacks))
	for _, bp := range buildpacks {
		bpType := bp.Type
		if bpType == "" {
			bpType = buildpack.TypeOCIEmbedded
		}
		parts = append(parts, bpType+" "+bp.Name)
	}
	return strings.Join(parts, ";")
}

func buildEnvList(envs map[string]string) []string {
	out := make([]string, 0, len(envs))
	for key, value := range envs {
		out = append(out, fmt.Sprintf(`%s=%s`, key, value))
	}
	return out
}

func imageTag(appCode, moduleName, version string) string {
	return fmt.Sprintf("%s/%s:%s", appCode, moduleName, version)
}
