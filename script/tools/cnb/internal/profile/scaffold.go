package profile

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*
var templatesFS embed.FS

// ScaffoldFile 是一个待写出的骨架文件。
type ScaffoldFile struct {
	Path    string // 相对仓库根
	Content string
	Mode    uint32
}

// Scaffold 用内嵌模板生成项目骨架（Dockerfile / build.sh / justfile）。
// 已存在的文件由调用方决定是否跳过，这里只负责渲染。
func Scaffold(name, dir string) ([]ScaffoldFile, error) {
	specs := []struct {
		tpl  string
		out  string
		mode uint32
	}{
		{"templates/Dockerfile.base.tmpl", dir + "/docker/Dockerfile.base", 0o644},
		{"templates/Dockerfile.run.tmpl", dir + "/docker/Dockerfile.run", 0o644},
		{"templates/Dockerfile.tmpl", dir + "/docker/Dockerfile", 0o644},
		{"templates/build.sh.tmpl", dir + "/build.sh", 0o755},
		{"templates/justfile.tmpl", dir + "/justfile", 0o644},
	}

	data := struct{ Name, Dir string }{Name: name, Dir: dir}
	out := make([]ScaffoldFile, 0, len(specs))
	for _, s := range specs {
		raw, err := templatesFS.ReadFile(s.tpl)
		if err != nil {
			return nil, fmt.Errorf("读取内嵌模板 %s: %w", s.tpl, err)
		}
		tpl, err := template.New(s.tpl).Parse(string(raw))
		if err != nil {
			return nil, fmt.Errorf("解析模板 %s: %w", s.tpl, err)
		}
		var b strings.Builder
		if err := tpl.Execute(&b, data); err != nil {
			return nil, fmt.Errorf("渲染模板 %s: %w", s.tpl, err)
		}
		out = append(out, ScaffoldFile{Path: s.out, Content: b.String(), Mode: s.mode})
	}
	return out, nil
}
