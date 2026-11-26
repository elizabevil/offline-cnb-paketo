package plan

import (
	"encoding/json"
	"fmt"

	"bksmartpacker/pkg/types"
)

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
			entry.ProcEntrypoints[process.Name] = []string{moduleName + "-" + process.Name}
		}
		artifact.AppArtifacts[moduleName] = entry
	}
	if len(artifact.AppArtifacts) == 0 {
		return nil, fmt.Errorf("no app artifacts generated")
	}
	return json.MarshalIndent(artifact, "", "  ")
}
