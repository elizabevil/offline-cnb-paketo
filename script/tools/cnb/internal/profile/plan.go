package profile

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/shell"
)

// Plan 是校验后的可执行计划，可序列化为 shell / JSON。
type Plan struct {
	Project       string   `json:"project" yaml:"project"`
	RepoRoot      string   `json:"repo_root" yaml:"repo_root"`
	ConfigPath    string   `json:"config" yaml:"config"`
	Dir           string   `json:"dir" yaml:"dir"`
	Platform      string   `json:"platform" yaml:"platform"`
	Context       string   `json:"context" yaml:"context"`
	BundleDir     string   `json:"bundle_dir" yaml:"bundle_dir"`
	BundleLayout  string   `json:"bundle_layout" yaml:"bundle_layout"`
	BuildpacksDir string   `json:"buildpacks_dir,omitempty" yaml:"buildpacks_dir,omitempty"`
	Runtime       string   `json:"runtime" yaml:"runtime"`
	Assets        []string `json:"assets" yaml:"assets"`
	// AssetDetails 是每个物料的详细信息（供描述输出与分析系统接入）。
	AssetDetails []AssetInfo `json:"asset_details,omitempty" yaml:"asset_details,omitempty"`
	Stages       []PlanStage `json:"stages" yaml:"stages"`
}

// AssetInfo 描述单个物料的解析结果与落盘状态。
type AssetInfo struct {
	Ref       string   `json:"ref" yaml:"ref"`
	ID        string   `json:"id" yaml:"id"`
	Version   string   `json:"version" yaml:"version"`
	Kind      string   `json:"kind" yaml:"kind"`
	Platforms []string `json:"platforms" yaml:"platforms"`
	SHA256    string   `json:"sha256,omitempty" yaml:"sha256,omitempty"`
	URL       string   `json:"url,omitempty" yaml:"url,omitempty"`
	Local     bool     `json:"local,omitempty" yaml:"local,omitempty"`
	Present   bool     `json:"present" yaml:"present"`
	Size      int64    `json:"size,omitempty" yaml:"size,omitempty"`
}

// PlanStage 是计划中的一个执行阶段（镜像或命令）。
// PlanStage 是计划中的一个执行阶段（镜像或命令）。
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
		BundleDir:    abs(mustDefault(p.Docker.BundleDir, lock.DefaultBundleDir)),
		BundleLayout: mustDefault(p.Docker.BundleLayout, lock.LayoutArchFirst),
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
	for _, id := range slices.Sorted(maps.Keys(p.Assets)) {
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
	plan.AssetDetails = buildAssetDetails(l, plan, platform)
	return plan, nil
}

// buildAssetDetails 汇总物料的解析信息与落盘状态。
func buildAssetDetails(l *lock.File, pl *Plan, platform string) []AssetInfo {
	out := make([]AssetInfo, 0, len(pl.Assets))
	for _, ref := range pl.Assets {
		a, err := lock.Find(l, ref)
		if err != nil {
			out = append(out, AssetInfo{Ref: ref})
			continue
		}
		info := AssetInfo{
			Ref: a.Ref(), ID: a.ID, Version: a.Version, Kind: a.Unpack.Kind,
			Platforms: assetPlatforms(a, platform), SHA256: a.SHA256(platform), Local: a.IsLocal(),
		}
		if u, ok := a.URL(platform); ok {
			info.URL = u
		}
		info.Present, info.Size = bundleState(pl.BundleDir, platform, a)
		out = append(out, info)
	}
	return out
}

// bundleState 判断物料产物是否已落盘，并返回归档大小。
func bundleState(bundleDir, platform string, a *lock.Asset) (bool, int64) {
	base := filepath.Join(bundleDir, platform, "binaries")
	for _, rel := range a.ExpectedPaths() {
		if _, err := os.Stat(filepath.Join(base, rel)); err != nil {
			return false, 0
		}
	}
	url, _ := a.URL(platform)
	archive := filepath.Join(bundleDir, platform, "archives", a.ID, a.Version, filepath.Base(url))
	if st, err := os.Stat(archive); err == nil {
		return true, st.Size()
	}
	return true, 0
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
			for _, k := range slices.Sorted(maps.Keys(st.Args)) {
				args = append(args, shell.Quote(k+"="+st.Args[k]))
			}
			line := fmt.Sprintf("image_build %s %s %s", shell.Quote(st.Name), shell.Quote(st.Dockerfile), shell.Quote(st.Tag))
			if len(args) > 0 {
				line += " " + strings.Join(args, " ")
			}
			b.WriteString(line + "\n")
		case "command":
			envs := make([]string, 0, len(st.Env))
			for _, k := range slices.Sorted(maps.Keys(st.Env)) {
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

// assetPlatforms 返回物料的平台列表。
// 本地构建物同样与架构相关：它是为当前平台编译的，声明为单平台。
func assetPlatforms(a *lock.Asset, platform string) []string {
	if a.IsLocal() {
		return []string{platform}
	}
	out := make([]string, 0, len(a.Platform))
	for k := range a.Platform {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
