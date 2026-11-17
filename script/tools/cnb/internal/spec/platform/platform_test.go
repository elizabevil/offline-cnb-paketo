package platform

import "testing"

func TestParseDefaults(t *testing.T) {
	env, warns, err := Parse(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("不应有告警: %v", warns)
	}
	if env.AppDir != DefaultAppDir || env.LayersDir != DefaultLayersDir {
		t.Fatalf("默认值错误: %+v", env)
	}
	if env.ExportMode != ExportLayout {
		t.Fatalf("默认导出方式应为 layout: %v", env.ExportMode)
	}
}

func TestParseValues(t *testing.T) {
	m := map[string]string{
		EnvOutputImage: "app:1",
		EnvExportMode:  "daemon",
		EnvRunImage:    "run:1",
		EnvRequired:    "buildpack go 1.0;buildpack nodejs 2.0",
		EnvSourceURL:   "file:///tmp/source.tgz",
	}
	env, _, err := Parse(func(k string) string { return m[k] })
	if err != nil {
		t.Fatal(err)
	}
	if env.OutputImage != "app:1" || env.RunImage != "run:1" || env.ExportMode != ExportDaemon {
		t.Fatalf("字段解析错误: %+v", env)
	}
	if len(env.Required) != 2 {
		t.Fatalf("required 解析错误: %v", env.Required)
	}
	if env.SourcePath() != "/tmp/source.tgz" {
		t.Fatalf("source path 错误: %q", env.SourcePath())
	}
}

func TestParseInvalidMode(t *testing.T) {
	m := map[string]string{EnvExportMode: "weird"}
	if _, _, err := Parse(func(k string) string { return m[k] }); err == nil {
		t.Fatal("非法导出方式应报错")
	}
}

func TestValidateLayoutNeedsDir(t *testing.T) {
	e := Env{ExportMode: ExportLayout}
	if err := e.Validate(); err == nil {
		t.Fatal("layout 缺目录应报错")
	}
}

func TestParseAliases(t *testing.T) {
	m := map[string]string{EnvBuildpackAliases: "bk-buildpack-go=paketo-buildpacks/go, x=y "}
	env, _, err := Parse(func(k string) string { return m[k] })
	if err != nil {
		t.Fatal(err)
	}
	if env.BuildpackAliases["bk-buildpack-go"] != "paketo-buildpacks/go" {
		t.Fatalf("别名解析错误: %v", env.BuildpackAliases)
	}
	if env.BuildpackAliases["x"] != "y" {
		t.Fatalf("空白未处理: %v", env.BuildpackAliases)
	}
	if empty, _, _ := Parse(func(string) string { return "" }); empty.BuildpackAliases != nil {
		t.Fatal("未设置时应为 nil")
	}
}
