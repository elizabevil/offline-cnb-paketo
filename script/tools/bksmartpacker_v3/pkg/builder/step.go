package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
	"github.com/samber/lo"

	"bksmartpacker/pkg/config"
)

type FeatureFlags struct {
	// EnableProxy 是否开启网络代理
	EnableProxy bool `json:"enableProxy"`
	// DisableCollectStatic 是否禁用静态文件收集
	DisableCollectStatic bool `json:"disableCollectStatic"`
}

type ModuleBuildStep struct {
	AppCode    string `json:"appCode"`
	ModuleName string `json:"module_name"`
	Version    string `json:"version"`
	// Src 源码目录或源码压缩包的绝对路径
	Src          string            `json:"src"`
	Procfile     string            `json:"procfile"`
	FeatureFlags FeatureFlags      `json:"featureFlags"`
	Buildpacks   []Buildpack       `json:"buildpacks"`
	Envs         map[string]string `json:"envs"`

	CacheRegistry string `json:"cacheRegistry"`
	RegistryAuth  string `json:"registryAuth"`
}

// RequiredBuildpacks ...
func (step ModuleBuildStep) RequiredBuildpacks() string {
	var parts []string
	for _, bp := range step.Buildpacks {
		version := bp.Version
		bpType := bp.Type
		// 未设置有效版本时，使用默认版本
		if version == "" {
			version = GetDefaultVersionByBPName(bp.Name)
		}
		if bpType == "" {
			bpType = GetDefaultTypeByBPName(bp.Name)
		}

		parts = append(parts, strings.Join([]string{bpType, bp.Name, "...", version}, " "))
	}
	return strings.Join(parts, ";")
}

// BuildpacksEnvs ...
func (step ModuleBuildStep) BuildpacksEnvs() []string {
	var envs []string
	for _, bp := range step.Buildpacks {
		switch bp.Name {
		case "bk-buildpack-python":
			envs = append(envs,
				"DISABLE_COLLECTSTATIC=1",
				fmt.Sprintf("PIP_VERSION=%s", config.G.Bp.Python.PipVersion),
				fmt.Sprintf("BUILDPACK_S3_BASE_URL=%s", config.G.Bp.Python.S3BaseURL),
				fmt.Sprintf("PIP_INDEX_URL=%s", config.G.Bp.Python.PipIndexURL),
				fmt.Sprintf("PIP_EXTRA_INDEX_URL=%s", config.G.Bp.Python.PipExtraIndexURL),
			)
			break
		case "bk-buildpack-nodejs":
			envs = append(
				envs,
				fmt.Sprintf("STDLIB_FILE_URL=%s", config.G.Bp.NodeJS.StdLibFileURL),
				fmt.Sprintf("S3_DOMAIN=%s", config.G.Bp.NodeJS.S3Domain),
				fmt.Sprintf("NPM_REGISTRY=%s", config.G.Bp.NodeJS.NpmRegistry),
			)
			break
		case "bk-buildpack-go":
			envs = append(envs, fmt.Sprintf("GO_BUCKET_URL=%s", config.G.Bp.Go.GoBucketURL))
			break
		}
	}
	return envs
}

// BuildEnvs ...
func (step ModuleBuildStep) BuildEnvs() []string {
	var envs []string
	for key, value := range step.Envs {
		envs = append(envs, fmt.Sprintf(`%s=%s`, key, value))
	}
	return envs
}

// OutputImage ...
func (step ModuleBuildStep) OutputImage() string {
	return fmt.Sprintf("%s/%s:%s", step.AppCode, step.ModuleName, step.Version)
}

// BuildSourceTarball ...
func BuildSourceTarball(src, procfileContent string, destPath string) error {
	// 当 Procfile 不存在时, 生成 Procfile 文件
	procfilePath := filepath.Join(src, "Procfile")
	// NOTE: 每次处理新模块时, 都重写 Procfile 文件, 以确保对应模块的 tarball 中的 Procfile 正确(避免不同模块共用 src, 导致生成的 Procfile 内容互相影响)
	if err := os.WriteFile(procfilePath, []byte(procfileContent), 0o755); err != nil {
		return errors.Wrap(err, "failed to write Procfile")
	}

	// 打包 tarball
	tarball := archiver.NewTarGz()
	entries, err := os.ReadDir(src)
	if err != nil {
		return errors.Wrap(err, "failed to archive src")
	}

	err = tarball.Archive(lo.Map(entries, func(item os.DirEntry, index int) string {
		return filepath.Join(src, item.Name())
	}), destPath)
	if err != nil {
		return errors.Wrap(err, "failed to archive src")
	}
	return tarball.Close()
}
