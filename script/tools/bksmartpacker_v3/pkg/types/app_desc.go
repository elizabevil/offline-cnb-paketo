package types

import (
	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
)

// AppInfo 应用信息
type AppInfo struct {
	Code string `yaml:"bkAppCode"`
	Name string `yaml:"bkAppName"`
}

// AppEnvVar defines the environment variable of bkapp
type AppEnvVar struct {
	// Name of the environment variable. Must be a C_IDENTIFIER.
	Name string `json:"Name" yaml:"name"`

	// Value of the environment variable
	Value string `json:"value" yaml:"value"`
}

// Process 进程模型
type Process struct {
	// Name of process
	Name string `json:"Name" yaml:"name"`

	// ProcCommand is the script command to start the process
	ProcCommand string `json:"procCommand" yaml:"procCommand"`
}

// AppConfig is bkapp related configuration, such as environment variables, etc.
type AppConfig struct {
	// List of environment variables to set in the container.
	Env []AppEnvVar `json:"env,omitempty" yaml:"env,omitempty"`
}

// Buildpack 构建工具
type Buildpack struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version,omitempty"`
}

// BuildConfig is the configuration related with application building, the platform
// support 3 types of build config currently: image, remote build by Dockerfile and
// remote build by buildpack.
type BuildConfig struct {
	Buildpacks []Buildpack `json:"buildpacks" yaml:"buildpacks"`
}

// BkAppSpec 云原生应用 Spec
type BkAppSpec struct {
	Processes     []Process   `json:"processes" yaml:"processes"`
	Configuration AppConfig   `json:"configuration" yaml:"configuration"`
	Build         BuildConfig `json:"build" yaml:"build"`
}

// ModuleDescription 模块描述文件
type ModuleDescription struct {
	Name      string    `json:"name" yaml:"name"`
	SourceDir string    `json:"sourceDir" yaml:"sourceDir"`
	Language  string    `json:"language" yaml:"language"`
	Spec      BkAppSpec `json:"spec" yaml:"spec"`
}

// ApplicationDescription 应用描述文件
type ApplicationDescription struct {
	SpecVersion int                 `json:"specVersion" yaml:"specVersion"`
	AppVersion  string              `json:"appVersion" yaml:"appVersion"`
	AppInfo     AppInfo             `json:"app" yaml:"app"`
	Modules     []ModuleDescription `json:"modules" yaml:"modules"`
}

// Validate 校验 ApplicationDescription 是否符合基本需求
func (appDesc *ApplicationDescription) Validate() error {
	if appDesc.SpecVersion == 0 {
		return errors.New("缺少 specVersion 字段, 请检查应用描述文件")
	}

	if appDesc.AppInfo.Code == "" {
		return errors.New("缺少 app.bkAppCode 字段, 请检查应用描述文件")
	}

	if appDesc.AppInfo.Name == "" {
		return errors.New("缺少 app.bkAppName 字段, 请检查应用描述文件")
	}

	if len(appDesc.Modules) == 0 {
		return errors.New("缺少 modules 字段, 请检查应用描述文件")
	}

	if err := appDesc.ValidateSpecVersion(); err != nil {
		return err
	}

	return appDesc.ValidateAppVersion()
}

func (appDesc *ApplicationDescription) String() string {
	return appDesc.AppInfo.Code
}

// ValidateSpecVersion 校验 SpecVersion
func (appDesc *ApplicationDescription) ValidateSpecVersion() error {
	if appDesc.SpecVersion < 3 {
		return errors.New("仅支持 specVersion >= 3 的应用描述文件")
	}
	return nil
}

// ValidateAppVersion 校验 AppVersion 是否符合 semver 格式
func (appDesc *ApplicationDescription) ValidateAppVersion() error {
	if appDesc.AppVersion == "" {
		return errors.New("缺少 appVersion 字段, 请检查应用描述文件")
	}

	if version, err := semver.Make(appDesc.AppVersion); err != nil {
		return errors.Wrapf(
			err,
			"appVersion 的版本号 '%s' 需符合语义化版本规范(Semantic Versioning), 即版本格式: 主版本号.次版本号.修订号",
			appDesc.AppVersion,
		)
	} else if err := version.Validate(); err != nil {
		return errors.Wrapf(err, "appVersion 的版本号 '%s' 需符合语义化版本规范(Semantic Versioning), 即版本格式: 主版本号.次版本号.修订号", appDesc.AppVersion)
	}
	return nil
}
