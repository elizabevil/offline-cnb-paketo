package profile

import (
	"maps"
	"slices"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// Summary 是配置的结构化视图（供 show 输出与外部消费）。
type Summary struct {
	Project  ProjectSummary   `json:"project" yaml:"project"`
	Build    BuildSummary     `json:"build" yaml:"build"`
	Assets   []AssetSummary   `json:"assets" yaml:"assets"`
	Images   []ImageSummary   `json:"images" yaml:"images"`
	Commands []CommandSummary `json:"commands" yaml:"commands"`
	Stages   []StageSummary   `json:"stages" yaml:"stages"`
}

// ProjectSummary 是项目标识。
type ProjectSummary struct {
	Name     string `json:"name" yaml:"name"`
	Dir      string `json:"dir" yaml:"dir"`
	Platform string `json:"platform" yaml:"platform"`
}

// BuildSummary 是构建相关设置。
type BuildSummary struct {
	Context      string `json:"context" yaml:"context"`
	BundleLayout string `json:"bundle_layout" yaml:"bundle_layout"`
	BundleDir    string `json:"bundle_dir,omitempty" yaml:"bundle_dir,omitempty"`
}

// AssetSummary 是单个物料的声明。
type AssetSummary struct {
	ID        string   `json:"id" yaml:"id"`
	Version   string   `json:"version,omitempty" yaml:"version,omitempty"`
	Platforms []string `json:"platforms,omitempty" yaml:"platforms,omitempty"`
}

// ImageSummary 是镜像阶段。
type ImageSummary struct {
	Name       string            `json:"name" yaml:"name"`
	Desc       string            `json:"desc,omitempty" yaml:"desc,omitempty"`
	Dockerfile string            `json:"dockerfile" yaml:"dockerfile"`
	Tag        string            `json:"tag" yaml:"tag"`
	Args       map[string]string `json:"args,omitempty" yaml:"args,omitempty"`
}

// CommandSummary 是命令阶段。
type CommandSummary struct {
	Name    string            `json:"name" yaml:"name"`
	Run     string            `json:"run" yaml:"run"`
	Workdir string            `json:"workdir" yaml:"workdir"`
	Env     map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
}

// StageSummary 是流程中的一步。
type StageSummary struct {
	Index int    `json:"index" yaml:"index"`
	Kind  string `json:"kind" yaml:"kind"`
	Name  string `json:"name" yaml:"name"`
}

// Summary 生成结构化视图。
func (p *File) Summary() Summary {
	s := Summary{
		Project: ProjectSummary{
			Name: p.Project.Name, Dir: p.Project.Dir, Platform: p.Project.Platform,
		},
		Build: BuildSummary{
			Context:      mustDefault(p.Docker.Context, "."),
			BundleLayout: mustDefault(p.Docker.BundleLayout, lock.LayoutArchFirst),
			BundleDir:    p.Docker.BundleDir,
		},
	}

	for _, id := range slices.Sorted(maps.Keys(p.Assets)) {
		a := p.Assets[id]
		s.Assets = append(s.Assets, AssetSummary{ID: id, Version: a.Version, Platforms: a.Platforms})
	}
	for _, name := range slices.Sorted(maps.Keys(p.Images)) {
		img := p.Images[name]
		s.Images = append(s.Images, ImageSummary{
			Name: name, Desc: img.Desc, Dockerfile: img.Dockerfile, Tag: img.Tag, Args: img.Args,
		})
	}
	for _, name := range slices.Sorted(maps.Keys(p.Commands)) {
		c := p.Commands[name]
		s.Commands = append(s.Commands, CommandSummary{
			Name: name, Run: c.Run, Workdir: mustDefault(c.Workdir, p.Project.Dir), Env: c.Env,
		})
	}
	for i, st := range p.Stages {
		kind, name := "image", st.Image
		if st.Command != "" {
			kind, name = "command", st.Command
		}
		s.Stages = append(s.Stages, StageSummary{Index: i + 1, Kind: kind, Name: name})
	}
	return s
}
