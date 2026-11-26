// Package main 提供 bksmartpacker 命令行入口。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"bksmartpacker/pkg/builder"
	"bksmartpacker/pkg/builder/executors"
	"bksmartpacker/pkg/cmd"
	"bksmartpacker/pkg/log"
	"bksmartpacker/pkg/smartpkg"
	"bksmartpacker/pkg/types"
)

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
var (
	src  = flag.String("src", "", "应用源码的路径, 支持相对路径(相对于工作目录)或绝对路径 \n 应用源码目录需要包含应用描述文件(app_desc.yaml)")
	dest = flag.String(
		"dest",
		"./s-mart.tar.gz",
		"s-mart 包的输出路径, 支持相对路径(相对于工作目录)或绝对路径; \n (s-mart 包是 gzip 压缩的 tarball, 建议以 .tar.gz 或 .tgz 作为命名后缀)",
	)
	help          = flag.BoolP("help", "h", false, "Show help")
	builderImage  = flag.String("builder-image", "", "cnb builder 镜像")
	cacheRegistry = flag.String("cache-registry", "", "缓存镜像仓库. 格式如: mirrors.tencent.com/bkapps")
	registryAuth  = flag.String(
		"registry-auth",
		"",
		"镜像仓库凭证信息. 格式如: '{\"mirrors.tencent.com\": \"Basic xxx\"}'",
	)
	inspectPkg = flag.String(
		"inspect",
		"",
		"只检查已有 s-mart 包的结构(不构建), 输出 path/size/version/层包摘要",
	)
	inspectFormat = flag.String("inspect-format", "text", "检查输出格式: text | json")
	buildEnvs     stringList
	buildModules  stringList
	buildEnvFiles stringList
	// packagingVersion v2 使用构建组 + artifact.json + 每组一个 image tar；
	// v1 保留旧的每模块 tgz，仅用于兼容旧平台/旧产物。
	packagingVersion = flag.String("packaging-version", builder.PackagingVersionV2, "s-mart 打包方案: v1(旧) | v2(默认，构建组+artifact)")
	// baseImageID 是平台侧 SMART_CNB_IMAGE_CONF 的 key；构建器不选择、不拉取 base image。
	baseImageID = flag.String("base-image-id", "default", "artifact.json 的 runtime.base_image_id（平台 base image 配置的 key）")
	// architecture 是目标镜像架构，必须与镜像内容一致；默认按 BKS_PLATFORM/宿主推断。
	architecture = flag.String("architecture", "", "artifact.json 的 runtime.architecture，默认按目标平台推断")
	// allowPartial 只用于调试；部分包缺少 app_desc 声明的其他模块，平台部署会失败。
	allowPartial = flag.Bool("allow-partial", false, "允许 --module 生成部分调试包（不可部署）")
)

// stringList 支持重复出现的字符串参数。
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func (s *stringList) Type() string { return "stringArray" }

func init() {
	flag.Var(&buildEnvs, "env", "注入构建容器的环境变量 KEY=VALUE，可重复")
	flag.Var(&buildModules, "module", "只构建指定模块，可重复；部分包默认不可部署，调试需配合 --allow-partial")
	flag.Var(&buildEnvFiles, "env-file", "dotenv 配置文件，可重复；后加载覆盖先加载")
	flag.Lookup("builder-image").DefValue = executors.DefaultCNBBuilderImage
}

func loadBuildParams() *types.Params {
	dest, err := filepath.Abs(*dest)
	if err != nil {
		panic(errors.Wrap(err, "failed to get absolute dest path"))
	}

	paramData := new(types.Params)
	paramData.Src = *src
	paramData.Dest = dest
	paramData.BuilderImage = *builderImage
	paramData.CacheRegistry = *cacheRegistry
	paramData.RegistryAuth = *registryAuth
	paramData.Envs = []string(buildEnvs)
	paramData.Modules = []string(buildModules)
	paramData.EnvFiles = []string(buildEnvFiles)
	paramData.PackagingVersion = *packagingVersion
	paramData.BaseImageID = *baseImageID
	paramData.Architecture = *architecture
	paramData.AllowPartial = *allowPartial

	return paramData
}

func main() {
	os.Exit(run())
}

func run() int {
	runtime.GOMAXPROCS(4)
	defer func() {
		if err := recover(); err != nil {
			log.Error("panic: ", err)
		}
	}()
	flag.Parse()
	if *help {
		flag.PrintDefaults()
		return 0
	}
	if *inspectPkg != "" {
		if err := runInspect(*inspectPkg, *inspectFormat); err != nil {
			log.Error(err)
			return 1
		}
		return 0
	}
	if *src == "" {
		log.Error(errors.New("src 不能为空"))
		flag.PrintDefaults()
		return 1
	}
	if err := execute(); err != nil {
		log.Error(err)
		return 1
	}
	return 0
}

func execute() error {
	// 输入参数解析到对象
	paramData := loadBuildParams()
	log.Info(fmt.Sprintf("\nparamData: %#v", paramData))

	// 业务逻辑
	log.Info("start BuildSMartPackage")
	if err := cmd.BuildSMartPackage(paramData); err != nil {
		return errors.Wrap(err, "failed to build smart package")
	}
	log.Info("BuildSMartPackage done")
	return nil
}

// runInspect 输出 s-mart 包的结构摘要。
func runInspect(path, format string) error {
	rep, err := smartpkg.Inspect(path)
	if err != nil {
		return errors.Wrap(err, "inspect s-mart package")
	}
	if format == "json" {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("path: %s\nsize: %d\nversion: %s\nhas_app_desc: %v\n", rep.Path, rep.Size, rep.Version, rep.HasDesc)
	fmt.Println("files:")
	for _, f := range rep.Files {
		fmt.Printf("  - %s\n", f)
	}
	fmt.Println("layers:")
	for _, l := range rep.Layers {
		fmt.Printf("  - %s (%d bytes) has_source=%v\n", l.Name, l.Size, l.HasSource)
		for _, e := range l.Entries {
			fmt.Printf("      %s\n", e)
		}
	}
	return nil
}
