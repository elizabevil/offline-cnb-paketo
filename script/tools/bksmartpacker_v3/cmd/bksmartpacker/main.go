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

	"bksmartpacker/pkg/builder/executors"
	"bksmartpacker/pkg/cmd"
	"bksmartpacker/pkg/log"
	"bksmartpacker/pkg/smartpkg"
	"bksmartpacker/pkg/types"
)

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
)

// stringList 支持重复出现的字符串参数。
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func (s *stringList) Type() string { return "stringArray" }

func init() {
	flag.Var(&buildEnvs, "env", "注入构建容器的环境变量 KEY=VALUE，可重复")
	flag.Var(&buildModules, "module", "只构建指定模块，可重复")
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

	return paramData
}

func main() {
	runtime.GOMAXPROCS(4)
	defer func() {
		if err := recover(); err != nil {
			log.Error("panic: ", err)
		}
	}()
	flag.Parse()
	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}
	if *inspectPkg != "" {
		if err := runInspect(*inspectPkg, *inspectFormat); err != nil {
			log.Error(err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if *src == "" {
		log.Error(errors.New("src 不能为空"))
		flag.PrintDefaults()
		os.Exit(1)
	}

	execute()
}

func execute() {
	// 输入参数解析到对象
	paramData := loadBuildParams()
	log.Info(fmt.Sprintf("\nparamData: %#v", paramData))

	// 业务逻辑
	log.Info("start BuildSMartPackage")
	err := cmd.BuildSMartPackage(paramData)
	if err != nil {
		log.Error(errors.Wrap(err, "failed to build smart package"))
		os.Exit(1)
	}
	log.Info("BuildSMartPackage done")
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
