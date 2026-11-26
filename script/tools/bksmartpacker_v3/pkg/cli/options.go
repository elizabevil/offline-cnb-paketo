package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/alecthomas/kong"

	"bksmartpacker/pkg/builder/executors"
	"bksmartpacker/pkg/plan"
	"bksmartpacker/pkg/types"
)

// Options 是 bksmartpacker 的命令行参数。
//
// 参数使用规则：
//   - --src/--dest 是本地路径；远程 URL 不在当前版本支持范围。
//   - --packaging-version 默认 v2：按构建组生成一个 image tar + artifact.json；
//     v1 仅用于兼容旧的每模块 tgz 产物。
//   - --base-image-id 是平台 SMART_CNB_IMAGE_CONF 的 key，默认 default；
//     构建器只声明该值，不负责拉取或选择 base image。
//   - --architecture 写入 artifact.json，默认按 BKS_PLATFORM 或宿主架构推断；
//     必须与实际构建出的镜像架构一致。
//   - --module 仅用于调试；默认禁止生成部分包，因为平台会按 app_desc 校验全部模块。
//   - --allow-partial 显式放行部分包；该产物不可部署。
//   - --env/--env-file 合并顺序：app_desc < env-file < --env。
type Options struct {
	Src              string        `name:"src" help:"应用源码路径，需包含 app_desc.yaml"`
	Dest             string        `name:"dest" default:"${defaultDest}" help:"s-mart 输出路径"`
	BuilderImage     string        `name:"builder-image" default:"${defaultBuilderImage}" help:"cnb builder 镜像"`
	CacheRegistry    string        `name:"cache-registry" help:"缓存镜像仓库，如 mirrors.tencent.com/bkapps"`
	RegistryAuth     string        `name:"registry-auth" help:"镜像仓库凭证 JSON，如 {\"host\":\"Basic xxx\"}"`
	Inspect          string        `name:"inspect" help:"只检查已有 s-mart 包，不构建"`
	InspectFormat    string        `name:"inspect-format" default:"${defaultInspectFormat}" enum:"text,json" help:"检查输出格式"`
	Envs             []string      `name:"env" sep:"none" help:"构建容器环境变量 KEY=VALUE，可重复"`
	EnvFiles         []string      `name:"env-file" sep:"none" help:"dotenv 配置文件，可重复；后加载覆盖先加载"`
	Modules          []string      `name:"module" sep:"none" help:"只构建指定模块，可重复；部分包默认不可部署"`
	PackagingVersion string        `name:"packaging-version" default:"${defaultPackagingVersion}" enum:"v1,v2" help:"打包方案：v1(旧) | v2(构建组+artifact)"`
	BaseImageID      string        `name:"base-image-id" default:"${defaultBaseImageID}" help:"artifact.json 的 runtime.base_image_id，平台 base image 配置的 key"`
	Architecture     string        `name:"architecture" help:"artifact.json 的 runtime.architecture，默认按目标平台推断"`
	AllowPartial     bool          `name:"allow-partial" help:"允许 --module 生成部分调试包（不可部署）"`
	Timeout          time.Duration `name:"timeout" default:"${defaultTimeout}" help:"单次构建总超时，如 90m、2h"`
}

func defaultOptions() Options {
	return Options{}
}

// Parse 解析命令行参数，默认值由 Kong 填充。
func Parse(args []string) (Options, error) {
	opts := defaultOptions()
	parser, err := kong.New(&opts,
		kong.Name("bksmartpacker"),
		kong.Description("离线 CNB s-mart 构建器"),
		kong.Vars{
			"defaultDest":             "./s-mart.tar.gz",
			"defaultBuilderImage":     executors.DefaultCNBBuilderImage,
			"defaultInspectFormat":    "text",
			"defaultPackagingVersion": plan.PackagingVersionV2,
			"defaultBaseImageID":      plan.DefaultBaseImageID,
			"defaultTimeout":          plan.DefaultBuildTimeout.String(),
		},
	)
	if err != nil {
		return opts, err
	}
	if _, err := parser.Parse(args); err != nil {
		return opts, err
	}
	return opts, nil
}

// Validate 校验跨字段规则。
func (o Options) Validate() error {
	if o.Inspect != "" {
		if o.Src != "" {
			return fmt.Errorf("--inspect and --src are mutually exclusive")
		}
		return nil
	}
	if o.Src == "" {
		return fmt.Errorf("--src is required")
	}
	if o.Timeout <= 0 {
		return fmt.Errorf("--timeout must be positive")
	}
	if len(o.Modules) > 0 && !o.AllowPartial {
		return fmt.Errorf("--module generates a partial package which cannot be deployed; use --allow-partial for debug builds")
	}
	return nil
}

// Params 转换为构建参数，并规范化输出路径。
func (o Options) Params() (*types.Params, error) {
	dest, err := filepath.Abs(o.Dest)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute dest path: %w", err)
	}
	return &types.Params{
		Src:              o.Src,
		Dest:             dest,
		BuilderImage:     o.BuilderImage,
		CacheRegistry:    o.CacheRegistry,
		RegistryAuth:     o.RegistryAuth,
		Envs:             o.Envs,
		EnvFiles:         o.EnvFiles,
		Modules:          o.Modules,
		PackagingVersion: o.PackagingVersion,
		BaseImageID:      o.BaseImageID,
		Architecture:     o.Architecture,
		AllowPartial:     o.AllowPartial,
		BuildTimeout:     o.Timeout,
	}, nil
}
