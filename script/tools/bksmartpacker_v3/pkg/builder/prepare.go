package builder

import (
	"fmt"
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

	plan := BuildPlan{
		AppCode:        appDesc.AppInfo.Code,
		AppDescContent: rawAppDesc,
		LogoFileName:   logoFileName,
		Src:            params.Src,
		Dest:           params.Dest,
		BuilderImage:   params.BuilderImage,
	}
	for _, moduleDesc := range appDesc.Modules {
		procfile, err := generateProcfile(moduleDesc)
		if err != nil {
			return nil, err
		}

		step := ModuleBuildStep{
			AppCode:    appDesc.AppInfo.Code,
			ModuleName: moduleDesc.Name,
			Version:    appDesc.AppVersion,
			Src:        filepath.Join(params.Src, moduleDesc.SourceDir),
			Procfile:   procfile,
			FeatureFlags: FeatureFlags{
				EnableProxy:          params.EnableProxy == "true",
				DisableCollectStatic: params.DisableCollectStatic == "true",
			},
			Buildpacks: generateModuleBuildpacks(params.Src, &moduleDesc),
			Envs:       mergeEnvs(moduleDesc.Spec.Configuration.Env, params.Envs),

			CacheRegistry: params.CacheRegistry,
			RegistryAuth:  params.RegistryAuth,
		}

		plan.Steps = append(plan.Steps, &step)
	}

	return &plan, nil
}

// BuildPlan 构建计划
type BuildPlan struct {
	// Dest s-mart 构建产物输出路径
	Dest string `json:"dest"`
	// 源码路径(从插件输入参数获取)
	Src            string             `json:"src"`
	AppCode        string             `json:"appCode"`
	AppDescContent string             `json:"appDesc"`
	LogoFileName   string             `json:"logoFileName"`
	Steps          []*ModuleBuildStep `json:"steps"`

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
func generateModuleBuildpacks(srcPath string, moduleDesc *types.ModuleDescription) []Buildpack {
	// app_desc.yaml 中配置了 spec.build.buildpacks
	if len(moduleDesc.Spec.Build.Buildpacks) > 0 {
		return lo.Map(moduleDesc.Spec.Build.Buildpacks, func(item types.Buildpack, index int) Buildpack {
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
	// 探测是否依赖 Apt
	if aptfileExists(filepath.Join(srcPath, moduleDesc.SourceDir)) {
		buildpacks = append(buildpacks, defaultBuildpacks["Apt"])
	}
	return buildpacks
}

// aptfileExists 检测 Aptfile 是否存在
func aptfileExists(modulePath string) bool {
	if _, err := os.Stat(filepath.Join(modulePath, "Aptfile")); err != nil {
		// 没有检测到有效的 Aptfile
		return false
	}
	return true
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
