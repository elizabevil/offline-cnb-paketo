package plan

import (
	"encoding/json"
	"fmt"
	"path"

	"bksmartpacker/pkg/types"
)

// processDir 是 lifecycle 生成 process symlink 的固定目录。
// artifact 的值会直接作为容器 command，因此必须使用绝对路径，避免依赖基础镜像 PATH。
const processDir = "/cnb/process"

// processEntrypoint 生成容器内可直接执行的 process 路径。
func processEntrypoint(moduleName, processName string) string {
	return path.Join(processDir, moduleName+"-"+processName)
}

// BuildArtifactJSON 根据构建组生成 artifact.json。
func BuildArtifactJSON(plan *BuildPlan) ([]byte, error) {
	artifact := types.ArtifactJSON{
		Version: types.ArtifactJSONVersion,
		Runtime: types.ArtifactRuntime{
			BaseImageID:  plan.BaseImageID,
			Architecture: plan.Architecture,
		},
		AppArtifacts: map[string]types.ArtifactModule{},
	}
	for _, group := range plan.Groups {
		for _, moduleName := range group.ModuleNames {
			artifact.AppArtifacts[moduleName] = types.ArtifactModule{
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
			entry.ProcEntrypoints[process.Name] = []string{processEntrypoint(moduleName, process.Name)}
		}
		artifact.AppArtifacts[moduleName] = entry
	}
	if len(artifact.AppArtifacts) == 0 {
		return nil, fmt.Errorf("no app artifacts generated")
	}
	return json.MarshalIndent(artifact, "", "  ")
}
