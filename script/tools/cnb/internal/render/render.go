// Package render 把执行计划渲染成不同格式；文本用 Go 模板，结构化输出保持键顺序。
package render

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"learn-cnb-paketo/tools/cnb/internal/profile"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

type row struct {
	Index      int      `json:"index" yaml:"index"`
	Kind       string   `json:"kind" yaml:"kind"`
	Name       string   `json:"name" yaml:"name"`
	Desc       string   `json:"desc,omitempty" yaml:"desc,omitempty"`
	Dockerfile string   `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"`
	Base       string   `json:"base,omitempty" yaml:"base,omitempty"`
	Context    string   `json:"context,omitempty" yaml:"context,omitempty"`
	BundleCtx  string   `json:"bundle_context,omitempty" yaml:"bundle_context,omitempty"`
	Tag        string   `json:"tag,omitempty" yaml:"tag,omitempty"`
	Workdir    string   `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	Run        string   `json:"run,omitempty" yaml:"run,omitempty"`
	Inputs     []string `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	ArgList    []string `json:"build_args,omitempty" yaml:"build_args,omitempty"`
	EnvList    []string `json:"env,omitempty" yaml:"env,omitempty"`
}

type view struct {
	Project      string   `json:"project" yaml:"project"`
	RepoRoot     string   `json:"repo_root" yaml:"repo_root"`
	ConfigPath   string   `json:"config" yaml:"config"`
	Dir          string   `json:"dir" yaml:"dir"`
	Platform     string   `json:"platform" yaml:"platform"`
	Context      string   `json:"context" yaml:"context"`
	BundleDir    string   `json:"bundle_dir" yaml:"bundle_dir"`
	BundleLayout string   `json:"bundle_layout" yaml:"bundle_layout"`
	Runtime      string   `json:"runtime" yaml:"runtime"`
	Assets       []string `json:"assets" yaml:"assets"`
	Stages       []row    `json:"stages" yaml:"stages"`
}

func toView(p *profile.Plan) view {
	v := view{
		Project: p.Project, RepoRoot: p.RepoRoot, ConfigPath: p.ConfigPath,
		Dir: p.Dir, Platform: p.Platform, Context: p.Context,
		BundleDir: p.BundleDir, BundleLayout: p.BundleLayout, Runtime: p.Runtime, Assets: p.Assets,
	}
	for i, st := range p.Stages {
		r := row{
			Index: i + 1, Kind: st.Kind, Name: st.Name, Desc: st.Desc,
			Dockerfile: st.Dockerfile, Base: st.Base, Context: st.Context, BundleCtx: st.BundleCtx,
			Tag: st.Tag, Workdir: st.Workdir, Run: st.Run,
		}
		for _, k := range sortedKeys(st.Args) {
			r.ArgList = append(r.ArgList, k+"="+st.Args[k])
		}
		for _, k := range sortedKeys(st.Env) {
			r.EnvList = append(r.EnvList, k+"="+st.Env[k])
		}
		v.Stages = append(v.Stages, r)
	}
	return v
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Text 用模板渲染人类可读摘要。
func Text(p *profile.Plan) (string, error) {
	raw, err := templateFS.ReadFile("templates/plan.tmpl")
	if err != nil {
		return "", err
	}
	tpl, err := template.New("plan").Funcs(template.FuncMap{
		"pad": func(s string, n int) string {
			if len(s) >= n {
				return s
			}
			return s + strings.Repeat(" ", n-len(s))
		},
	}).Parse(string(raw))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, toView(p)); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n") + "\n", nil
}

// Shell 交给执行器的片段。
func Shell(p *profile.Plan) string { return p.Shell() }

// JSON 缩进输出；键顺序由结构体字段顺序决定。
func JSON(p *profile.Plan) ([]byte, error) {
	return json.MarshalIndent(toView(p), "", "  ")
}

// YAML 输出；键顺序由结构体字段顺序决定（不经过 map）。
func YAML(p *profile.Plan) ([]byte, error) {
	return yaml.Marshal(toView(p))
}

// Format 是统一入口。
func Format(p *profile.Plan, format string) (string, error) {
	switch format {
	case "text", "":
		return Text(p)
	case "shell":
		return Shell(p), nil
	case "json":
		b, err := JSON(p)
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	case "yaml":
		b, err := YAML(p)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "", fmt.Errorf("unsupported format %q (text|shell|json|yaml)", format)
	}
}
