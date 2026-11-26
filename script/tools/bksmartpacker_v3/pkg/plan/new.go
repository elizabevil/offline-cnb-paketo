package plan

import (
	"fmt"
	"path/filepath"

	"github.com/samber/lo"

	"bksmartpacker/pkg/buildpack"
	"bksmartpacker/pkg/env"
	"bksmartpacker/pkg/types"
)

// New 根据 app_desc 与参数生成构建计划。
func New(appDesc types.ApplicationDescription, rawAppDesc, logoFileName string, params *types.Params) (*BuildPlan, error) {
	plan := newBuildPlan(appDesc, rawAppDesc, logoFileName, params)
	if plan.PackagingVersion != PackagingVersionV1 && plan.PackagingVersion != PackagingVersionV2 {
		return nil, fmt.Errorf("invalid packaging version %q: must be v1 or v2", plan.PackagingVersion)
	}
	if err := prepareSteps(appDesc, params, &plan); err != nil {
		return nil, err
	}
	if plan.PackagingVersion == PackagingVersionV2 {
		if err := buildGroups(appDesc.Modules, &plan); err != nil {
			return nil, err
		}
	}
	return &plan, nil
}

func newBuildPlan(appDesc types.ApplicationDescription, rawAppDesc, logoFileName string, params *types.Params) BuildPlan {
	packagingVersion := params.PackagingVersion
	if packagingVersion == "" {
		packagingVersion = PackagingVersionV2
	}
	baseImageID := params.BaseImageID
	if baseImageID == "" {
		baseImageID = DefaultBaseImageID
	}
	return BuildPlan{
		AppCode:          appDesc.AppInfo.Code,
		AppDescContent:   rawAppDesc,
		LogoFileName:     logoFileName,
		Src:              params.Src,
		Dest:             params.Dest,
		BuilderImage:     params.BuilderImage,
		PackagingVersion: packagingVersion,
		BaseImageID:      baseImageID,
		Architecture:     params.Architecture,
		ModuleProcesses:  map[string][]ModuleProcess{},
	}
}

func prepareSteps(appDesc types.ApplicationDescription, params *types.Params, p *BuildPlan) error {
	for _, moduleDesc := range appDesc.Modules {
		procfile := ""
		if p.PackagingVersion == PackagingVersionV1 {
			var err error
			procfile, err = generateProcfile(moduleDesc)
			if err != nil {
				return err
			}
		}
		envs, err := env.MergeModule(moduleDesc.Spec.Configuration.Env, params.EnvFiles, params.Envs)
		if err != nil {
			return err
		}
		step := ModuleBuildStep{
			AppCode:       appDesc.AppInfo.Code,
			ModuleName:    moduleDesc.Name,
			Version:       appDesc.AppVersion,
			Src:           filepath.Join(params.Src, moduleDesc.SourceDir),
			Procfile:      procfile,
			Buildpacks:    buildpack.ForModule(&moduleDesc),
			Envs:          envs,
			CacheRegistry: params.CacheRegistry,
			RegistryAuth:  params.RegistryAuth,
		}
		p.Steps = append(p.Steps, &step)
		p.ModuleProcesses[moduleDesc.Name] = lo.Map(moduleDesc.Spec.Processes, func(proc types.Process, _ int) ModuleProcess {
			return ModuleProcess{Name: proc.Name, Command: proc.ProcCommand}
		})
	}
	return nil
}
