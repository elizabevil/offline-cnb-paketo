package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// BuildpackTypeOCIEmbedded 表示 buildpack 已嵌入 builder 镜像。
const BuildpackTypeOCIEmbedded = "oci-embedded"

// ModuleBuildStep 是一个模块的 v1 构建单元。
type ModuleBuildStep struct {
	AppCode    string `json:"appCode"`
	ModuleName string `json:"module_name"`
	Version    string `json:"version"`
	// Src 源码目录或源码压缩包的绝对路径
	Src        string            `json:"src"`
	Procfile   string            `json:"procfile"`
	Buildpacks []Buildpack       `json:"buildpacks"`
	Envs       map[string]string `json:"envs"`

	CacheRegistry string `json:"cacheRegistry"`
	RegistryAuth  string `json:"registryAuth"`
}

// RequiredBuildpacks 生成 lifecycle 的 REQUIRED_BUILDPACKS。
// 字段格式保留 "type name"，本地 lifecycle 只按 name/别名匹配。
func (step ModuleBuildStep) RequiredBuildpacks() string {
	var parts []string
	for _, bp := range step.Buildpacks {
		bpType := bp.Type
		if bpType == "" {
			bpType = BuildpackTypeOCIEmbedded
		}
		parts = append(parts, bpType+" "+bp.Name)
	}
	return strings.Join(parts, ";")
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

	err = tarball.Archive(lo.Map(entries, func(item os.DirEntry, _ int) string {
		return filepath.Join(src, item.Name())
	}), destPath)
	if err != nil {
		return errors.Wrap(err, "failed to archive src")
	}
	return tarball.Close()
}
