package builder

import (
	"maps"
	"strings"

	"bksmartpacker/pkg/types"
)

// buildGroups 按 sourceDir + buildpacks 合并模块，生成 v2 构建组。
func buildGroups(modules []types.ModuleDescription, plan *BuildPlan) error {
	appProcfile, err := GenerateAppProcfile(modules)
	if err != nil {
		return err
	}
	groups := map[string]*ModuleBuildGroup{}
	order := make([]string, 0)
	for i, moduleDesc := range modules {
		step := plan.Steps[i]
		key := groupKey(moduleDesc.SourceDir, step.Buildpacks)
		group, ok := groups[key]
		if !ok {
			group = &ModuleBuildGroup{
				AppCode:       plan.AppCode,
				ModuleName:    moduleDesc.Name,
				ModuleNames:   []string{moduleDesc.Name},
				Version:       step.Version,
				SourceDir:     moduleDesc.SourceDir,
				Src:           step.Src,
				Procfile:      appProcfile,
				Buildpacks:    step.Buildpacks,
				Envs:          map[string]string{},
				CacheRegistry: step.CacheRegistry,
				RegistryAuth:  step.RegistryAuth,
			}
			groups[key] = group
			order = append(order, key)
		} else {
			group.ModuleNames = append(group.ModuleNames, moduleDesc.Name)
		}
		// 组内 env 取并集，后出现的模块覆盖同名键。
		maps.Copy(group.Envs, step.Envs)
	}
	for _, key := range order {
		group := groups[key]
		group.OutputImageTarName = group.ModuleName + ImageTarExt
		plan.Groups = append(plan.Groups, group)
	}
	return nil
}

func groupKey(sourceDir string, buildpacks []Buildpack) string {
	parts := make([]string, 0, 1+len(buildpacks)*3)
	parts = append(parts, sourceDir)
	for _, bp := range buildpacks {
		parts = append(parts, bp.Type, bp.Name, bp.Version)
	}
	return strings.Join(parts, "|")
}
