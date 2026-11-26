package types

// Params 自定义参数, 蓝盾传参风格, bool 值也是字符串
type Params struct {
	// Src 源码路径
	Src string `json:"src"`
	// Dest 构建 s-mart 输出路径
	Dest string `json:"dest"`
	// BuilderImage cnb builder 镜像
	BuilderImage string `json:"builderImage"`
	// CacheRegistry 缓存仓库
	CacheRegistry string `json:"cacheRegistry"`
	// RegistryAuth 镜像仓库凭证信息
	RegistryAuth string `json:"registryAuth"`
	// Envs 注入构建容器的额外环境变量（KEY=VALUE），覆盖 app_desc 同名项
	Envs []string `json:"envs,omitempty"`
	// EnvFiles dotenv 配置文件，按顺序覆盖 app_desc 的环境变量
	EnvFiles []string `json:"envFiles,omitempty"`
	// Modules 只构建指定模块（为空表示全部）
	Modules []string `json:"modules,omitempty"`
	// PackagingVersion v1(旧) / v2(构建组 + artifact.json)
	PackagingVersion string `json:"packagingVersion,omitempty"`
	// BaseImageID 运行时基础镜像 ID
	BaseImageID string `json:"baseImageId,omitempty"`
	// Architecture 目标镜像架构
	Architecture string `json:"architecture,omitempty"`
	// AllowPartial 允许生成只含部分模块的调试包（不可部署）
	AllowPartial bool `json:"allowPartial,omitempty"`
}
