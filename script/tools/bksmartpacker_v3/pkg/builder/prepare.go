package builder

import (
	"fmt"
	"path/filepath"

	"github.com/samber/lo"

	"bksmartpacker/pkg/log"
	"bksmartpacker/pkg/types"
	"bksmartpacker/pkg/utils"
)

// PrepareBuildPlan 解析参数，输出构建计划。
func PrepareBuildPlan(params *types.Params) (*BuildPlan, error) {
	appDesc, rawAppDesc, err := loadAppDesc(params.Src)
	if err != nil {
		return nil, err
	}
	if err := appDesc.Validate(); err != nil {
		return nil, err
	}
	if err := filterModules(&appDesc, params.Modules, params.AllowPartial); err != nil {
		return nil, err
	}

	logoFileName := utils.DetectLogoFile(params.Src)
	if logoFileName == "" {
		log.Warn("缺少 logo 文件", "expect", "app_desc.yaml 同级 logo.png")
	} else {
		log.Info("检测到 logo 文件", "file", logoFileName)
	}

	plan := newBuildPlan(appDesc, rawAppDesc, logoFileName, params)
	if plan.PackagingVersion != PackagingVersionV1 && plan.PackagingVersion != PackagingVersionV2 {
		return nil, fmt.Errorf("invalid packaging version %q: must be v1 or v2", plan.PackagingVersion)
	}
	if err := prepareSteps(appDesc, params, &plan); err != nil {
		return nil, err
	}
	// v2 按 sourceDir + buildpacks 分组；v1 保留每模块独立构建。
	if plan.PackagingVersion == PackagingVersionV2 {
		if err := buildGroups(appDesc.Modules, &plan); err != nil {
			return nil, err
		}
	}
	return &plan, nil
}

// prepareSteps 为每个模块生成 v1 Step，并记录模块进程。
func prepareSteps(appDesc types.ApplicationDescription, params *types.Params, plan *BuildPlan) error {
	for _, moduleDesc := range appDesc.Modules {
		procfile := ""
		if plan.PackagingVersion == PackagingVersionV1 {
			var err error
			procfile, err = generateProcfile(moduleDesc)
			if err != nil {
				return err
			}
		}
		envs, err := mergeModuleEnvs(moduleDesc.Spec.Configuration.Env, params.EnvFiles, params.Envs)
		if err != nil {
			return err
		}
		step := ModuleBuildStep{
			AppCode:       appDesc.AppInfo.Code,
			ModuleName:    moduleDesc.Name,
			Version:       appDesc.AppVersion,
			Src:           filepath.Join(params.Src, moduleDesc.SourceDir),
			Procfile:      procfile,
			Buildpacks:    generateModuleBuildpacks(params.Src, &moduleDesc),
			Envs:          envs,
			CacheRegistry: params.CacheRegistry,
			RegistryAuth:  params.RegistryAuth,
		}
		plan.Steps = append(plan.Steps, &step)
		plan.ModuleProcesses[moduleDesc.Name] = lo.Map(moduleDesc.Spec.Processes, func(p types.Process, _ int) ModuleProcess {
			return ModuleProcess{Name: p.Name, Command: p.ProcCommand}
		})
	}
	return nil
}
