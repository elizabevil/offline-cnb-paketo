// Package render 把执行计划渲染成不同格式；文本用 Go 模板，结构化输出保持键顺序。
package render

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"learn-cnb-paketo/tools/cnb/internal/profile"
	"learn-cnb-paketo/tools/cnb/internal/version"
)

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
	Project      string      `json:"project" yaml:"project"`
	RepoRoot     string      `json:"repo_root" yaml:"repo_root"`
	ConfigPath   string      `json:"config" yaml:"config"`
	Dir          string      `json:"dir" yaml:"dir"`
	Platform     string      `json:"platform" yaml:"platform"`
	Context      string      `json:"context" yaml:"context"`
	BundleDir    string      `json:"bundle_dir" yaml:"bundle_dir"`
	BundleLayout string      `json:"bundle_layout" yaml:"bundle_layout"`
	Runtime      string      `json:"runtime" yaml:"runtime"`
	Assets       []string    `json:"assets" yaml:"assets"`
	AssetDetails []AssetView `json:"asset_details,omitempty" yaml:"asset_details,omitempty"`
	Stages       []row       `json:"stages" yaml:"stages"`

	// 构建信息
	ToolVersion string `json:"tool_version" yaml:"tool_version"`
	ToolCommit  string `json:"tool_commit,omitempty" yaml:"tool_commit,omitempty"`
	BuilderTag  string `json:"builder_tag,omitempty" yaml:"builder_tag,omitempty"`
	RunTag      string `json:"run_tag,omitempty" yaml:"run_tag,omitempty"`
}

func toView(p *profile.Plan) view {
	v := view{
		Project: p.Project, RepoRoot: p.RepoRoot, ConfigPath: p.ConfigPath,
		Dir: p.Dir, Platform: p.Platform, Context: p.Context,
		BundleDir: p.BundleDir, BundleLayout: p.BundleLayout, Runtime: p.Runtime, Assets: p.Assets,
		AssetDetails: assetViews(p),
		ToolVersion:  version.Get().Short(),
		ToolCommit:   shortCommit(version.Get().Commit),
	}
	for _, st := range p.Stages {
		if st.Kind != "image" {
			continue
		}
		if strings.HasSuffix(st.Name, "builder") {
			v.BuilderTag = st.Tag
		}
		if st.Name == "run" {
			v.RunTag = st.Tag
		}
	}
	for i, st := range p.Stages {
		r := row{
			Index: i + 1, Kind: st.Kind, Name: st.Name, Desc: st.Desc,
			Dockerfile: st.Dockerfile, Base: st.Base, Context: st.Context, BundleCtx: st.BundleCtx,
			Tag: st.Tag, Workdir: st.Workdir, Run: st.Run,
		}
		for _, k := range slices.Sorted(maps.Keys(st.Args)) {
			r.ArgList = append(r.ArgList, k+"="+st.Args[k])
		}
		for _, k := range slices.Sorted(maps.Keys(st.Env)) {
			r.EnvList = append(r.EnvList, k+"="+st.Env[k])
		}
		v.Stages = append(v.Stages, r)
	}
	return v
}

// Shell 交给执行器的片段。
func Shell(p *profile.Plan) string { return p.Shell() }

// JSON 缩进输出；键顺序由结构体字段顺序决定。
func JSON(p *profile.Plan) ([]byte, error) {
	return json.MarshalIndent(toView(p), "", "  ")
}

// YAML 输出；键顺序由结构体字段顺序决定（不经过 map）。
func YAML(p *profile.Plan) ([]byte, error) {
	data, err := yaml.Marshal(toView(p))
	if err != nil {
		return nil, err
	}
	return compactYAML(data)
}

// inlineMaxItems / inlineMaxLen 控制"短序列折成一行"的阈值。
const (
	inlineMaxItems = 4
	inlineMaxLen   = 72
)

// compactYAML 把短且元素为标量的序列折成 flow 风格（[a, b]），
// 大结构仍保持缩进展开。
func compactYAML(data []byte) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return data, nil // 解析失败时退回原始输出
	}
	inlineShortSequences(&node)
	inlineSmallMaps(&node)
	return yaml.Marshal(&node)
}

// inlineShortSequences 递归处理序列节点。
func inlineShortSequences(n *yaml.Node) {
	for _, c := range n.Content {
		inlineShortSequences(c)
	}
	if n.Kind != yaml.SequenceNode || len(n.Content) == 0 || len(n.Content) > inlineMaxItems {
		return
	}
	total := 0
	for _, c := range n.Content {
		if c.Kind != yaml.ScalarNode {
			return
		}
		total += len(c.Value) + 2
	}
	if total > inlineMaxLen {
		return
	}
	n.Style = yaml.FlowStyle
}

// inlineSmallMaps 把序列里的小映射折成一行（{k: v, …}）。
func inlineSmallMaps(n *yaml.Node) {
	for _, c := range n.Content {
		inlineSmallMaps(c)
	}
	if n.Kind != yaml.SequenceNode {
		return
	}
	for _, item := range n.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) > inlineMaxItems*2 {
			return // 有一项不满足就整体保持展开
		}
		total := 0
		for _, v := range item.Content {
			if v.Kind != yaml.ScalarNode && v.Style != yaml.FlowStyle {
				return
			}
			total += len(v.Value) + 4
		}
		if total > inlineMaxLen*2 {
			return
		}
	}
	for _, item := range n.Content {
		item.Style = yaml.FlowStyle
	}
}

// Format 是统一入口。
func Format(p *profile.Plan, format string) (string, error) {
	switch format {
	case "table":
		return Table(p), nil
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
		return "", fmt.Errorf("unsupported format %q (table|shell|json|yaml)", format)
	}
}

// shortCommit 截取提交号前 8 位，便于阅读。
func shortCommit(c string) string {
	if len(c) <= 8 {
		return c
	}
	return c[:8]
}

// AssetView 是物料在描述输出中的形态。
type AssetView struct {
	Ref       string   `json:"ref" yaml:"ref"`
	Version   string   `json:"version" yaml:"version"`
	Kind      string   `json:"kind" yaml:"kind"`
	Platforms []string `json:"platforms" yaml:"platforms"`
	Local     bool     `json:"local,omitempty" yaml:"local,omitempty"`
	Present   bool     `json:"present" yaml:"present"`
	Size      int64    `json:"size,omitempty" yaml:"size,omitempty"`
	SHA256    string   `json:"sha256,omitempty" yaml:"sha256,omitempty"`
}

// assetViews 把 Plan 的物料详情转成输出视图。
func assetViews(p *profile.Plan) []AssetView {
	out := make([]AssetView, 0, len(p.AssetDetails))
	for _, a := range p.AssetDetails {
		out = append(out, AssetView{
			Ref: a.Ref, Version: a.Version, Kind: a.Kind, Platforms: a.Platforms,
			Local: a.Local, Present: a.Present, Size: a.Size, SHA256: a.SHA256,
		})
	}
	return out
}

// Summary 是精简后的计划视图（供日常查看）。
type Summary struct {
	Project  string       `json:"project" yaml:"project"`
	Platform string       `json:"platform" yaml:"platform"`
	Runtime  string       `json:"runtime" yaml:"runtime"`
	Stages   []StageBrief `json:"stages" yaml:"stages"`
	Assets   []AssetBrief `json:"assets" yaml:"assets"`
}

// StageBrief 用"输入 → 动作 → 输出"描述一个阶段。
type StageBrief struct {
	Index  int    `json:"index" yaml:"index"`
	Kind   string `json:"kind" yaml:"kind"`
	Name   string `json:"name" yaml:"name"`
	Input  string `json:"input" yaml:"input"`
	Action string `json:"action" yaml:"action"`
	Output string `json:"output,omitempty" yaml:"output,omitempty"`
}

// AssetBrief 是物料的精简信息。
type AssetBrief struct {
	Ref       string   `json:"ref" yaml:"ref"`
	Kind      string   `json:"kind" yaml:"kind"`
	Platforms []string `json:"platforms" yaml:"platforms"`
	Present   bool     `json:"present" yaml:"present"`
}

// CompactPlan 输出精简计划（yaml/json 均可）。
func CompactPlan(p *profile.Plan, format string) (string, error) {
	if format == "table" || format == "" {
		return Table(p), nil
	}
	s := Summary{Project: p.Project, Platform: p.Platform, Runtime: p.Runtime}
	for i, st := range p.Stages {
		b := StageBrief{Index: i + 1, Kind: st.Kind, Name: st.Name}
		switch st.Kind {
		case "image":
			b.Input = shortPath(p.RepoRoot, st.Dockerfile) + " (context " + shortPath(p.RepoRoot, st.Context) + ")"
			b.Action = "docker build"
			b.Output = st.Tag
		case "command":
			b.Input = shortPath(p.RepoRoot, st.Workdir)
			b.Action = "run " + truncate(st.Run, 60)
		}
		s.Stages = append(s.Stages, b)
	}
	for _, a := range p.AssetDetails {
		s.Assets = append(s.Assets, AssetBrief{Ref: a.Ref, Kind: a.Kind, Platforms: a.Platforms, Present: a.Present})
	}
	if format == "json" {
		b, err := json.MarshalIndent(s, "", "  ")
		return string(b) + "\n", err
	}
	b, err := yaml.Marshal(s)
	if err != nil {
		return "", err
	}
	out, err := compactYAML(b)
	return string(out), err
}

// shortPath 返回相对仓库根的路径，便于阅读。
func shortPath(root, path string) string {
	if root == "" || path == "" {
		return path
	}
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

// truncate 截断过长文本。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
