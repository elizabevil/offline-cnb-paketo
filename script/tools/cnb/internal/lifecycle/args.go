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
	a.flag("layers", c.Layers)
	a.flag("app", c.App)
	a.flag("platform", c.Platform)
	return a
}

// detectArgs 生成 detector 参数。
func (c Config) detectArgs() []string {
	a := c.common()
	order := c.Order
	if c.Required != "" {
		order = c.Layers + "/" + OrderFileName
	}
	a.flag("order", order)
	return a
}

// buildArgs 生成 builder 参数。
func (c Config) buildArgs() []string { return c.common() }

// analyzeArgs 生成 analyzer 参数（仅 daemon 模式需要）。
func (c Config) analyzeArgs() []string {
	var a args
	a.flag("layers", c.Layers)
	a.flag("run-image", c.RunImage)
	a.boolFlag("daemon", true)
	a.positional(c.OutputImage)
	return a
}

// exportArgs 生成 exporter 参数。
func (c Config) exportArgs() []string {
	var a args
	a.flag("layers", c.Layers)
	a.flag("app", c.App)
	if c.ExportLayout {
		dir := c.LayoutDir
		if dir == "" {
			dir = defaultLayoutDir
		}
		a.boolFlag("layout", true)
		a.flag("layout-dir", dir)
	} else {
		a.boolFlag("daemon", true)
	}
	a.flag("run-image", c.RunImage)
	a.flag("cache-image", c.CacheImage)
	a.positional(c.OutputImage)
	return a
}
