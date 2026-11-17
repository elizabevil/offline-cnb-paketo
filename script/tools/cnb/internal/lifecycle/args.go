package lifecycle

// args 以链式方式拼装命令行参数，避免散落的字符串拼接。
type args []string

// flag 添加 "-name=value"；value 为空时跳过（可选项）。
func (a *args) flag(name, value string) {
	if value == "" {
		return
	}
	*a = append(*a, "-"+name+"="+value)
}

// boolFlag 仅在启用时添加 "-name"。
func (a *args) boolFlag(name string, on bool) {
	if on {
		*a = append(*a, "-"+name)
	}
}

// positional 追加位置参数（如镜像名）。
func (a *args) positional(value string) {
	if value != "" {
		*a = append(*a, value)
	}
}

// common 是 detect/build 共用的目录参数。
func (c Config) common() args {
	var a args
	a.flag("layers", c.Paths.Layers)
	a.flag("app", c.Paths.App)
	a.flag("platform", c.Paths.Platform)
	return a
}

// detectArgs 生成 detector 参数。
func (c Config) detectArgs() []string {
	a := c.common()
	order := c.Build.Order
	if c.Build.Required != "" {
		order = c.Paths.Layers + "/" + OrderFileName
	}
	a.flag("order", order)
	return a
}

// buildArgs 生成 builder 参数。
func (c Config) buildArgs() []string { return c.common() }

// analyzeArgs 生成 analyzer 参数。
// analyzer 始终走 daemon：它要把 run image 的元数据写进 analyzed.toml，
// 而 layout 模式下 analyzer 只会在 layout 目录里找镜像（尚未导入时必然失败）。
func (c Config) analyzeArgs() []string {
	var a args
	a.flag("layers", c.Paths.Layers)
	a.flag("run-image", c.Export.RunImage)
	a.boolFlag("daemon", true)
	a.positional(c.Export.OutputImage)
	return a
}

// layoutDir 返回 OCI layout 输出目录。
func (c Config) layoutDir() string {
	if c.Export.LayoutDir != "" {
		return c.Export.LayoutDir
	}
	return defaultLayoutDir
}

// exportArgs 生成 exporter 参数。
func (c Config) exportArgs() []string {
	var a args
	a.flag("layers", c.Paths.Layers)
	a.flag("app", c.Paths.App)
	switch c.Export.Mode {
	case ExportLayout, "":
		a.boolFlag("layout", true)
		a.flag("layout-dir", c.layoutDir())
	case ExportDaemon:
		a.boolFlag("daemon", true)
	case ExportNone:
		// 不导出：调用方不应执行 exporter
	}
	a.flag("run-image", c.Export.RunImage)
	a.flag("cache-image", c.Export.CacheImage)
	a.positional(c.Export.OutputImage)
	return a
}
