package types

// Params 自定义参数, 蓝盾传参风格, bool 值也是字符串
type Params struct {
	// Src 源码路径
	Src string `json:"src"`
	// Dest 构建 s-mart 输出路径
	Dest string `json:"dest"`
	// EnableProxy 是否开启网络代理
	EnableProxy string `json:"enableProxy"`
	// DisableCollectStatic 是否禁用静态文件收集
	DisableCollectStatic string `json:"disableCollectStatic"`
	// BuilderImage cnb builder 镜像
	BuilderImage string `json:"builderImage"`
	// CacheRegistry 缓存仓库
	CacheRegistry string `json:"cacheRegistry"`
	// RegistryAuth 镜像仓库凭证信息
	RegistryAuth string `json:"registryAuth"`
	// Envs 注入构建容器的额外环境变量（KEY=VALUE），覆盖 app_desc 同名项
	Envs []string `json:"envs,omitempty"`
	// Modules 只构建指定模块（为空表示全部）
	Modules []string `json:"modules,omitempty"`
}
