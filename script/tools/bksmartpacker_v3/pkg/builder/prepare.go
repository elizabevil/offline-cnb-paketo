package builder

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"gopkg.in/yaml.v3"

	"bksmartpacker/pkg/log"
	"bksmartpacker/pkg/types"
	"bksmartpacker/pkg/utils"
)

// PrepareBuildPlan 解析参数, 输出构建计划
func PrepareBuildPlan(params *types.Params) (*BuildPlan, error) {
	appDesc, rawAppDesc, err := loadAppDesc(params.Src)
	if err != nil {
		return nil, err
	}

	if err := appDesc.Validate(); err != nil {
		return nil, err
	}

	if len(params.Modules) > 0 && !params.AllowPartial {
		return nil, fmt.Errorf("--module generates a partial package which cannot be deployed; use --allow-partial for debug builds")
	}
	if len(params.Modules) > 0 {
		want := lo.SliceToMap(params.Modules, func(name string) (string, bool) { return name, true })
		filtered := make([]types.ModuleDescription, 0, len(params.Modules))
		for _, m := range appDesc.Modules {
			if want[m.Name] {
				filtered = append(filtered, m)
				delete(want, m.Name)
			}
		}
		if len(want) > 0 {
			return nil, fmt.Errorf("module not found: %v", lo.Keys(want))
		}
		appDesc.Modules = filtered
	}

	logoFileName := utils.DetectLogoFile(params.Src)
	if logoFileName == "" {
		log.Warn("请在 app_desc.yaml 同级的目录下添加 logo.png 文件")
	} else {
		log.Info("检测到 logo 文件: ", logoFileName)
	}

	packagingVersion := params.PackagingVersion
	if packagingVersion == "" {
		packagingVersion = PackagingVersionV2
	}
	if packagingVersion != PackagingVersionV1 && packagingVersion != PackagingVersionV2 {
		return nil, fmt.Errorf("invalid packaging version %q: must be v1 or v2", packagingVersion)
	}
	baseImageID := params.BaseImageID
	if baseImageID == "" {
		baseImageID = "default"
	}

	plan := BuildPlan{
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
	if err := prepareSteps(appDesc, params, packagingVersion, &plan); err != nil {
		return nil, err
	}

	if packagingVersion == PackagingVersionV2 {
		if err := buildGroups(appDesc.Modules, &plan); err != nil {
			return nil, err
		}
	}

	return &plan, nil
}

// prepareSteps 为每个模块生成 v1 Step，并记录模块进程。
func prepareSteps(appDesc types.ApplicationDescription, params *types.Params, packagingVersion string, plan *BuildPlan) error {
	for _, moduleDesc := range appDesc.Modules {
		procfile := ""
		if packagingVersion == PackagingVersionV1 {
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
		keyParts := []string{moduleDesc.SourceDir}
		for _, bp := range step.Buildpacks {
			keyParts = append(keyParts, bp.Type, bp.Name, bp.Version)
		}
		key := strings.Join(keyParts, "|")
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
		group.OutputImageTarName = group.ModuleName + ".tar"
		plan.Groups = append(plan.Groups, group)
	}
	return nil
}

// BuildPlan 构建计划
type BuildPlan struct {
	// Dest s-mart 构建产物输出路径
	Dest string `json:"dest"`
	// 源码路径(从插件输入参数获取)
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

// 加载应用描述文件
func loadAppDesc(src string) (types.ApplicationDescription, string, error) {
	if err := ensureAppDescYaml(src); err != nil {
		return types.ApplicationDescription{}, "", errors.Wrap(err, "app_desc.yaml not found")
	}

	// 读取 app_desc.yaml
	appDesc := types.ApplicationDescription{}
	data, err := os.ReadFile(filepath.Join(src, "app_desc.yaml"))
	if err != nil {
		return types.ApplicationDescription{}, "", errors.Wrap(err, "failed to read app_desc.yaml")
	}
	if err := yaml.Unmarshal(data, &appDesc); err != nil {
		return types.ApplicationDescription{}, "", errors.Wrap(err, "failed to parse app_desc.yaml")
	}
	return appDesc, string(data), nil
}

// 保证应用描述文件在路径 src/app_desc.yaml 下
func ensureAppDescYaml(src string) error {
	_, err := os.Stat(filepath.Join(src, "app_desc.yaml"))
	// 如果 app_desc.yaml 不存在, 则尝试探测 app_desc.yml
	if err != nil && os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(src, "app_desc.yml")); err != nil {
			return err
		}

		// 将 app_desc.yml 重命名为 app_desc.yaml
		if err := utils.Copy(filepath.Join(src, "app_desc.yml"), filepath.Join(src, "app_desc.yaml")); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(src, "app_desc.yml")); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

// 生成构建时需要的 procfile
func generateProcfile(moduleDesc types.ModuleDescription) (string, error) {
	procfile := make(map[string]string)
	for _, process := range moduleDesc.Spec.Processes {
		command := process.ProcCommand
		if command == "" {
			panic(fmt.Sprintf("进程 %s 缺少有效的 procCommand 值", process.Name))
		}
		procfile[process.Name] = command
	}

	procfileData, err := yaml.Marshal(procfile)
	if err != nil {
		return "", err
	}
	return string(procfileData), nil
}

// generateModuleBuildpacks 生成构建时需要的 buildpacks.
// srcPath 是应用源码路径, 通过 filepath.Join(srcPath, moduleDesc.SourceDir) 可以获取模块的源码路径, 从而探测是否依赖 Apt buildpack
func generateModuleBuildpacks(_ string, moduleDesc *types.ModuleDescription) []Buildpack {
	// app_desc.yaml 中配置了 spec.build.buildpacks
	if len(moduleDesc.Spec.Build.Buildpacks) > 0 {
		return lo.Map(moduleDesc.Spec.Build.Buildpacks, func(item types.Buildpack, _ int) Buildpack {
			return Buildpack{
				Name:    item.Name,
				Version: item.Version,
			}
		})
	}

	buildpacks := make([]Buildpack, 0)
	// 根据 moduleDesc.Language 选择默认的 buildpack
	defaultBuildpacks := GetDefaultBuildpacks()
	if bp, ok := defaultBuildpacks[moduleDesc.Language]; ok {
		buildpacks = append(buildpacks, bp)
	}
	return buildpacks
}

// mergeEnvs 合并 app_desc 环境变量与命令行注入项，后者覆盖前者。
func mergeEnvs(declared []types.AppEnvVar, extra []string) map[string]string {
	out := lo.Associate(declared, func(item types.AppEnvVar) (string, string) {
		return item.Name, item.Value
	})
	for _, kv := range extra {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

// mergeModuleEnvs 合并 app_desc、env-file、CLI 三层，后者覆盖前者。
func mergeModuleEnvs(declared []types.AppEnvVar, envFiles []string, extra []string) (map[string]string, error) {
	out := mergeEnvs(declared, nil)
	for _, path := range envFiles {
		values, err := LoadEnvFile(path)
		if err != nil {
			return nil, err
		}
		maps.Copy(out, values)
	}
	for _, kv := range extra {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k != "" {
			out[k] = v
		}
	}
	return out, nil
}
