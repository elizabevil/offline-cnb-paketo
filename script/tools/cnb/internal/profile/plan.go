package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/shell"
)

// Plan 是校验后的可执行计划，可序列化为 shell / JSON。
type Plan struct {
	Project       string      `json:"project" yaml:"project"`
	RepoRoot      string      `json:"repo_root" yaml:"repo_root"`
	ConfigPath    string      `json:"config" yaml:"config"`
	Dir           string      `json:"dir" yaml:"dir"`
	Platform      string      `json:"platform" yaml:"platform"`
	Context       string      `json:"context" yaml:"context"`
	BundleDir     string      `json:"bundle_dir" yaml:"bundle_dir"`
	BundleLayout  string      `json:"bundle_layout" yaml:"bundle_layout"`
	BuildpacksDir string      `json:"buildpacks_dir,omitempty" yaml:"buildpacks_dir,omitempty"`
	Runtime       string      `json:"runtime" yaml:"runtime"`
	Assets        []string    `json:"assets" yaml:"assets"`
	Stages        []PlanStage `json:"stages" yaml:"stages"`
}

// PlanStage 是计划中的一步。
type PlanStage struct {
	Kind          string            `json:"kind" yaml:"kind"` // fetch | image | command
	Name          string            `json:"name" yaml:"name"`
	Desc          string            `json:"desc,omitempty" yaml:"desc,omitempty"`
	Base          string            `json:"base,omitempty" yaml:"base,omitempty"`
	Context       string            `json:"context,omitempty" yaml:"context,omitempty"`
	BundleCtx     string            `json:"bundle_context,omitempty" yaml:"bundle_context,omitempty"`
	BuildpacksCtx string            `json:"buildpacks_context,omitempty" yaml:"buildpacks_context,omitempty"`
	Dockerfile    string            `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"`
	Tag           string            `json:"tag,omitempty" yaml:"tag,omitempty"`
	Args          map[string]string `json:"args,omitempty" yaml:"args,omitempty"`
	Run           string            `json:"run,omitempty" yaml:"run,omitempty"`
	Workdir       string            `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	Env           map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
}

// BuildPlan 校验配置并生成计划。
func (p *File) BuildPlan(platform string, l *lock.File) (*Plan, error) {
	if errs := p.Validate(platform, l); len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	abs := func(rel string) string {
		if filepath.IsAbs(rel) {
			return rel
		}
		return filepath.Join(repoRoot(), rel)
	}
	plan := &Plan{
		Project:      p.Project.Name,
		RepoRoot:     repoRoot(),
		Dir:          abs(p.Project.Dir),
		Platform:     platform,
		Context:      abs(mustDefault(p.Docker.Context, ".")),
		BundleDir:    abs(mustDefault(p.Docker.BundleDir, "bundle")),
		BundleLayout: mustDefault(p.Docker.BundleLayout, "arch-first"),
		Runtime:      detectRuntime(),
		BuildpacksDir: func() string {
			if p.Docker.BuildpacksDir != "" {
				return abs(p.Docker.BuildpacksDir)
			}
			// 仓库级共享 buildpacks，按格式分组（paketo / heroku / …）
			if dir := abs(mustDefault(p.Docker.BuildpacksContext, "buildpacks")); dirExists(dir) {
				return dir
			}
			return filepath.Join(abs(p.Project.Dir), "docker", "buildpacks")
		}(),
		Assets: p.Refs(platform),
	}

	// 物料版本 → <ID>_VERSION，供 Dockerfile 拼路径
	assetArgs := map[string]string{}
	for _, id := range sortedKeys(p.Assets) {
		ver := p.Assets[id].Version
		if ver == "" {
			if a, err := lock.Find(l, id); err == nil {
				ver = a.Version
			}
		}
		if ver != "" {
			assetArgs[id] = ver
		}
	}

	tags := map[string]string{}
	for _, st := range p.Stages {
		if st.Image != "" {
			img := p.Images[st.Image]
			tag := expandVars(resolveRefs(img.Tag, tags), platform)
			tags[st.Image] = tag

			merged := map[string]string{}
			for _, id := range img.InjectVersions {
				if v, ok := assetArgs[id]; ok {
					merged[versionVar(id)] = v
				}
			}
			for k, v := range img.Args {
				merged[k] = expandVars(resolveRefs(v, tags), platform)
			}
			if img.Base != "" {
				if _, ok := merged["BASE_IMAGE"]; !ok {
					merged["BASE_IMAGE"] = img.Base
				}
			}
			plan.Stages = append(plan.Stages, PlanStage{
				Kind: "image", Name: st.Image, Desc: img.Desc, Base: img.Base,
				Context: plan.Context, BundleCtx: plan.BundleDir,
				BuildpacksCtx: abs(mustDefault(p.Docker.BuildpacksContext, "buildpacks")),
				Dockerfile:    filepath.Join(plan.Context, img.Dockerfile), Tag: tag, Args: merged,
			})
			continue
		}
		c := p.Commands[st.Command]
		env := map[string]string{}
		for k, v := range c.Env {
			env[k] = expandVars(resolveRefs(v, tags), platform)
		}
		plan.Stages = append(plan.Stages, PlanStage{
			Kind: "command", Name: st.Command, Run: c.Run,
			Workdir: abs(mustDefault(c.Workdir, p.Project.Dir)), Env: env,
		})
	}
	return plan, nil
}

// Shell 渲染为 run.sh 兼容的 shell 片段（保持既有契约）。
func (pl *Plan) Shell() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# generated from %s\n", pl.Project)
	fmt.Fprintf(&b, "PROFILE_NAME=%s\n", shell.Quote(pl.Project))
	fmt.Fprintf(&b, "PROFILE_DIR=%s\n", shell.Quote(pl.Dir))
	fmt.Fprintf(&b, "PLATFORM=%s\n", shell.Quote(pl.Platform))
	fmt.Fprintf(&b, "PROFILE_CONTEXT=%s\n", shell.Quote(pl.Context))
	fmt.Fprintf(&b, "BUNDLE_LAYOUT=%s\n", shell.Quote(pl.BundleLayout))

	if len(pl.Assets) > 0 {
		q := make([]string, 0, len(pl.Assets))
		for _, a := range pl.Assets {
			q = append(q, shell.Quote(a))
		}
		fmt.Fprintf(&b, "fetch_assets %s\n", strings.Join(q, " "))
	}
	for _, st := range pl.Stages {
		switch st.Kind {
		case "image":
			args := make([]string, 0, len(st.Args))
			for _, k := range sortedKeysStr(st.Args) {
				args = append(args, shell.Quote(k+"="+st.Args[k]))
			}
			line := fmt.Sprintf("image_build %s %s %s", shell.Quote(st.Name), shell.Quote(st.Dockerfile), shell.Quote(st.Tag))
			if len(args) > 0 {
				line += " " + strings.Join(args, " ")
			}
			b.WriteString(line + "\n")
		case "command":
			envs := make([]string, 0, len(st.Env))
			for _, k := range sortedKeysStr(st.Env) {
				envs = append(envs, shell.Quote(k+"="+st.Env[k]))
			}
			line := fmt.Sprintf("run_command %s %s %s", shell.Quote(st.Name), shell.Quote(st.Run), shell.Quote(st.Workdir))
			if len(envs) > 0 {
				line += " " + strings.Join(envs, " ")
			}
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func sortedKeysStr(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 稳定输出
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// Text 渲染为人类可读的摘要（dry-run 默认输出）。
func (pl *Plan) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "流水线：%s（%s）\n", pl.Project, pl.Platform)
	fmt.Fprintf(&b, "目录：%s  上下文：%s\n", pl.Dir, pl.Context)

	fmt.Fprintf(&b, "\n物料（%d）\n", len(pl.Assets))
	if len(pl.Assets) == 0 {
		b.WriteString("  （无）\n")
	}
	for _, a := range pl.Assets {
		fmt.Fprintf(&b, "  · %s\n", a)
	}

	fmt.Fprintf(&b, "\n阶段（%d）\n", len(pl.Stages))
	for i, st := range pl.Stages {
		switch st.Kind {
		case "image":
			fmt.Fprintf(&b, "  %d. image   %-10s → %s\n", i+1, st.Name, st.Tag)
		case "command":
			fmt.Fprintf(&b, "  %d. command %-10s cd %s && %s\n", i+1, st.Name, st.Workdir, st.Run)
		}
	}
	return b.String()
}

// detectRuntime 探测可用的容器运行时（与 image-build.sh 的优先级一致）。
func detectRuntime() string {
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	return "none"
}

// dirExists 判断目录是否存在。
func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
