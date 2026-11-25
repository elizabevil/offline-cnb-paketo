package builder

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"bksmartpacker/pkg/types"

	"gopkg.in/yaml.v3"
)

// ModuleBuildGroup 是共享同一构建产物的模块集合。
type ModuleBuildGroup struct {
	AppCode            string            `json:"appCode"`
	ModuleName         string            `json:"moduleName"`
	ModuleNames        []string          `json:"moduleNames"`
	Version            string            `json:"version"`
	SourceDir          string            `json:"sourceDir"`
	Src                string            `json:"src"`
	Procfile           string            `json:"procfile"`
	Buildpacks         []Buildpack       `json:"buildpacks"`
	Envs               map[string]string `json:"envs"`
	OutputImageTarName string            `json:"outputImageTarName"`

	CacheRegistry string `json:"cacheRegistry"`
	RegistryAuth  string `json:"registryAuth"`
}

// RequiredBuildpacks 生成 lifecycle 的 REQUIRED_BUILDPACKS。
func (g ModuleBuildGroup) RequiredBuildpacks() string {
	var parts []string
	for _, bp := range g.Buildpacks {
		bpType := bp.Type
		if bpType == "" {
			bpType = "oci-embedded"
		}
		parts = append(parts, bpType+" "+bp.Name)
	}
	return strings.Join(parts, ";")
}

// BuildEnvs 返回平台环境变量。
func (g ModuleBuildGroup) BuildEnvs() []string {
	var envs []string
	for key, value := range g.Envs {
		envs = append(envs, fmt.Sprintf(`%s=%s`, key, value))
	}
	return envs
}

// OutputImage 是构建组主模块的镜像 tag。
func (g ModuleBuildGroup) OutputImage() string {
	return fmt.Sprintf("%s/%s:%s", g.AppCode, g.ModuleName, g.Version)
}

// ModuleProcess 是模块内的一个进程。
type ModuleProcess struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

// ArtifactJSON 是 v2 s-mart 的 artifact.json。
type ArtifactJSON struct {
	Version      string                    `json:"version"`
	Runtime      ArtifactRuntime           `json:"runtime"`
	AppArtifacts map[string]ArtifactModule `json:"app_artifacts"`
}

type ArtifactRuntime struct {
	BaseImageID  string `json:"base_image_id"`
	Architecture string `json:"architecture"`
}

type ArtifactModule struct {
	ImageTar        string              `json:"image_tar"`
	ProcEntrypoints map[string][]string `json:"proc_entrypoints"`
}

var (
	moduleNameRe  = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	processNameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
)

// GenerateAppProcfile 生成应用级 Procfile，key 为 <module>-<process>。
func GenerateAppProcfile(modules []types.ModuleDescription) (string, error) {
	procfile := make(map[string]string)
	for _, module := range modules {
		if !moduleNameRe.MatchString(module.Name) {
			return "", fmt.Errorf("invalid module name %q: must match %s", module.Name, moduleNameRe.String())
		}
		for _, process := range module.Spec.Processes {
			if process.Name == "" || process.ProcCommand == "" {
				return "", fmt.Errorf("module %s process %q has empty name/command", module.Name, process.Name)
			}
			if !processNameRe.MatchString(process.Name) {
				return "", fmt.Errorf("invalid process name %q in module %s: must match %s", process.Name, module.Name, processNameRe.String())
			}
			key := module.Name + "-" + process.Name
			procfile[key] = process.ProcCommand
		}
	}
	data, err := yaml.Marshal(procfile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// BuildArtifactJSON 根据构建组生成 artifact.json。
func BuildArtifactJSON(plan *BuildPlan) ([]byte, error) {
	artifact := ArtifactJSON{
		Version: "1.0",
		Runtime: ArtifactRuntime{
			BaseImageID:  plan.BaseImageID,
			Architecture: plan.Architecture,
		},
		AppArtifacts: map[string]ArtifactModule{},
	}
	for _, group := range plan.Groups {
		for _, moduleName := range group.ModuleNames {
			artifact.AppArtifacts[moduleName] = ArtifactModule{
				ImageTar:        group.OutputImageTarName,
				ProcEntrypoints: map[string][]string{},
			}
		}
	}
	for moduleName, processes := range plan.ModuleProcesses {
		entry, ok := artifact.AppArtifacts[moduleName]
		if !ok {
			return nil, fmt.Errorf("module %s is not assigned to any build group", moduleName)
		}
		for _, process := range processes {
			entry.ProcEntrypoints[process.Name] = []string{moduleName + "-" + process.Name}
		}
		artifact.AppArtifacts[moduleName] = entry
	}
	if len(artifact.AppArtifacts) == 0 {
		return nil, fmt.Errorf("no app artifacts generated")
	}
	return json.MarshalIndent(artifact, "", "  ")
}
