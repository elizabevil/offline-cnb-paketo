package profile

import (
	"embed"
	"learn-cnb-paketo/tools/cnb/internal/errs"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
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
		{"templates/Dockerfile.base.tmpl", dir + "/docker/Dockerfile.base", fsutil.FileMode},
		{"templates/Dockerfile.run.tmpl", dir + "/docker/Dockerfile.run", fsutil.FileMode},
		{"templates/Dockerfile.tmpl", dir + "/docker/Dockerfile", fsutil.FileMode},
		{"templates/build.sh.tmpl", dir + "/build.sh", fsutil.DirMode},
		{"templates/justfile.tmpl", dir + "/justfile", fsutil.FileMode},
	}

	data := struct{ Name, Dir string }{Name: name, Dir: dir}
	out := make([]ScaffoldFile, 0, len(specs))
	for _, s := range specs {
		raw, err := templatesFS.ReadFile(s.tpl)
		if err != nil {
			return nil, errs.Wrap("read template "+s.tpl, err)
		}
		tpl, err := template.New(s.tpl).Parse(string(raw))
		if err != nil {
			return nil, errs.Wrap("parse template "+s.tpl, err)
		}
		var b strings.Builder
		if err := tpl.Execute(&b, data); err != nil {
			return nil, errs.Wrap("render template "+s.tpl, err)
		}
		out = append(out, ScaffoldFile{Path: s.out, Content: b.String(), Mode: s.mode})
	}
	return out, nil
}
