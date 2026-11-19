package lifecycle

// args collects lifecycle flags in order. Methods make the "may be empty"
// distinction explicit instead of hiding it in a shared helper.
type args []string

// optional appends "-name=value" and skips empty values (truly optional flags).
func (a *args) optional(name, value string) {
	if value != "" {
		a.always(name, value)
	}
}

// always appends "-name=value" even when empty: a missing required value then
// fails in the lifecycle instead of being silently dropped.
func (a *args) always(name, value string) {
	*a = append(*a, "-"+name+"="+value)
}

// toggle appends a boolean flag.
func (a *args) toggle(name string) {
	*a = append(*a, "-"+name)
}

// positional appends a positional argument (for example the output image).
func (a *args) positional(value string) {
	if value != "" {
		*a = append(*a, value)
	}
}

// common is shared by detect and build.
func (c Config) common() args {
	var a args
	a.optional("layers", c.Paths.Layers)
	a.optional("app", c.Paths.App)
	a.optional("platform", c.Paths.Platform)
	return a
}

// detectArgs builds detector flags.
func (c Config) detectArgs() []string {
	a := c.common()
	order := c.Build.Order
	if c.Build.Required != "" {
		order = c.Paths.Layers + "/" + OrderFileName
	}
	a.optional("order", order)
	return a
}

// buildArgs builds builder flags.
func (c Config) buildArgs() []string { return c.common() }

// analyzeArgs builds analyzer flags.
//
// The analyzer always talks to the daemon: it records run image metadata in
// analyzed.toml, while layout mode would look for the image inside a layout
// directory that has not been populated yet.
func (c Config) analyzeArgs() []string {
	var a args
	a.optional("layers", c.Paths.Layers)
	a.optional("run-image", c.Export.RunImage)
	if c.usesLayout() {
		a.toggle("layout")
		a.optional("layout-dir", c.layoutDir())
	} else {
		a.toggle("daemon")
	}
	a.positional(c.Export.OutputImage)
	return a
}

// layoutDir returns the OCI layout output directory.
func (c Config) layoutDir() string {
	if c.Export.LayoutDir != "" {
		return c.Export.LayoutDir
	}
	return defaultLayoutDir
}

// usesLayout reports whether the export mode writes an OCI layout.
func (c Config) usesLayout() bool {
	return c.Export.Mode == ExportLayout || c.Export.Mode == ""
}

// exportArgs builds exporter flags.
func (c Config) exportArgs() []string {
	var a args
	a.optional("layers", c.Paths.Layers)
	a.optional("app", c.Paths.App)
	switch {
	case c.usesLayout():
		a.toggle("layout")
		a.optional("layout-dir", c.layoutDir())
	case c.Export.Mode == ExportDaemon:
		a.toggle("daemon")
		// Layout mode takes run image data from analyzed.toml, so only daemon
		// mode passes -run-image.
		a.optional("run-image", c.Export.RunImage)
	case c.Export.Mode == ExportNone:
		// Nothing to export; the caller must not run the exporter.
	}
	a.optional("cache-image", c.Export.CacheImage)
	a.positional(c.Export.OutputImage)
	return a
}
