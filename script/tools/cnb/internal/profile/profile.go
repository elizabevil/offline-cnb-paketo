// Package profile 描述一条完整流水线：物料、镜像、命令与构建参数。
//
// 配置按用途分区（见 config/projects/*.toml）：
//
//	[project]   项目身份与默认平台
//	[build]     docker 构建参数（上下文、bundle 位置与布局）
//	[assets.*]  物料引用（版本、适用平台）
//	[images.*]  镜像构建（dockerfile、tag、args、依赖）
//	[commands.*]命令阶段（打包、测试），可与镜像形成依赖
//
// 命名即标识：`[images.base]` 的名字就是 base，依赖用 after = ["base"] 表达。
package profile

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/strutil"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// File 是流水线配置的根结构。
type File struct {
	Schema   int                    `toml:"schema"`
	Project  Project                `toml:"project"`
	Docker   Docker                 `toml:"docker"`
	Assets   map[string]AssetSpec   `toml:"assets"`
	Images   map[string]ImageSpec   `toml:"images"`
	Commands map[string]CommandSpec `toml:"commands"`
	Stages   []Stage                `toml:"stage"`
}

// Project 描述项目身份。
type Project struct {
	Name     string `toml:"name"`
	Dir      string `toml:"dir"`      // 项目目录，相对仓库根
	Platform string `toml:"platform"` // 默认平台，可被 --platform 覆盖
}

// Docker 描述 docker build 的公共参数（与“构建脚本/构建工具”无关）。
type Docker struct {
	Context      string `toml:"context"`    // docker build 上下文，相对仓库根
	BundleDir    string `toml:"bundle_dir"` // 物料目录，相对仓库根，默认 bundle
	BundleLayout string `toml:"bundle_layout"`
	// BuildpacksDir 用于生成 builder metadata 的 buildpacks 来源目录
	// （相对仓库根；默认 <project>/docker/buildpacks）
	BuildpacksDir string `toml:"buildpacks_dir"`
	// BuildpacksContext 是 buildpack 资产所在目录（仓库根相对），
	// 构建时以命名上下文 buildpacks 注入
	BuildpacksContext string `toml:"buildpacks_context"` // arch-first | kind-first
}

// AssetSpec 是对某个物料的引用。
type AssetSpec struct {
	Version   string   `toml:"version"`   // 省略 = lock 中 default 版本
	Platforms []string `toml:"platforms"` // 省略 = 全部平台
}

func (a AssetSpec) ref(id string) string {
	if a.Version == "" {
		return id
	}
	return id + "@" + a.Version
}

// ImageSpec 描述一个镜像构建阶段。
type ImageSpec struct {
	Desc       string   `toml:"desc"`       // 用途说明，供人与自动化理解
	Dockerfile string   `toml:"dockerfile"` // 相对构建上下文
	Tag        string   `toml:"tag"`
	Base       string   `toml:"base"`      // 基础镜像，自动注入 BASE_IMAGE
	Platforms  []string `toml:"platforms"` // 该镜像适用的平台
	// InjectVersions 声明"把哪些物料的版本注入为 <ID>_VERSION 构建参数"。
	// 只影响构建参数，不表示镜像依赖该物料的二进制。
	InjectVersions []string          `toml:"inject_versions"`
	Args           map[string]string `toml:"args"`
}

// CommandSpec 描述一个命令阶段（打包、测试等）。
type CommandSpec struct {
	Run     string            `toml:"run"`
	Workdir string            `toml:"workdir"` // 工作目录，相对仓库根，默认 = project.dir
	Env     map[string]string `toml:"env"`
}

// Stage 是流程中的一步：引用一个镜像或一条命令，顺序即数组顺序。
type Stage struct {
	Image   string `toml:"image"`
	Command string `toml:"command"`
}

var tagRefRe = regexp.MustCompile(`\$\{image\.([A-Za-z0-9_.-]+)\.tag\}`)

// Load 读取并解析流水线配置。
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p File
	if err := toml.Unmarshal(data, &p); err != nil {
		return nil, errs.Wrap("parse toml", err)
	}
	return &p, nil
}

// Name 返回项目名。
func (p *File) Name() string { return p.Project.Name }

// DefaultPlatform 返回配置里的默认平台。
func (p *File) DefaultPlatform() string { return p.Project.Platform }

//nolint:funlen // TODO(protocol-layers): 按 image/command 分别渲染
func (p *File) render(l *lock.File, platform string) string {
	tags := map[string]string{}
	// 物料版本自动注入为 <ID>_VERSION，供 Dockerfile 拼路径，避免版本写两处
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
	var b strings.Builder

	fmt.Fprintf(&b, "# generated from %s\n", p.Project.Name)
	fmt.Fprintf(&b, "PROFILE_NAME=%s\n", strutil.QuoteShell(p.Project.Name))
	fmt.Fprintf(&b, "PROFILE_DIR=%s\n", strutil.QuoteShell(filepath.ToSlash(p.Project.Dir)))
	fmt.Fprintf(&b, "PLATFORM=%s\n", strutil.QuoteShell(platform))
	fmt.Fprintf(&b, "PROFILE_CONTEXT=%s\n", strutil.QuoteShell(filepath.ToSlash(mustDefault(p.Docker.Context, "."))))
	fmt.Fprintf(&b, "BUNDLE_DIR=%s\n", strutil.QuoteShell(filepath.ToSlash(filepath.Join(repoRoot(), mustDefault(p.Docker.BundleDir, lock.DefaultBundleDir)))))
	fmt.Fprintf(&b, "BUNDLE_LAYOUT=%s\n", strutil.QuoteShell(mustDefault(p.Docker.BundleLayout, lock.LayoutArchFirst)))
	fmt.Fprintf(&b, "export BUNDLE_DIR BUNDLE_LAYOUT\n")

	// 物料：按名字排序，输出显式 id@version
	var refs []string
	for _, id := range slices.Sorted(maps.Keys(p.Assets)) {
		a := p.Assets[id]
		if len(a.Platforms) > 0 && !containsStr(a.Platforms, platform) {
			continue
		}
		refs = append(refs, strutil.QuoteShell(a.ref(id)))
	}
	if len(refs) > 0 {
		fmt.Fprintf(&b, "fetch_assets %s\n", strings.Join(refs, " "))
	}

	for _, st := range p.Stages {
		if st.Image != "" {
			img := p.Images[st.Image]
			tag := expandVars(resolveRefs(img.Tag, tags), lock.Platform(platform))
			tags[st.Image] = tag

			merged := map[string]string{}
			for _, id := range img.InjectVersions {
				if v, ok := assetArgs[id]; ok {
					merged[versionVar(id)] = v
				}
			}
			maps.Copy(merged, img.Args)
			if img.Base != "" {
				if _, ok := merged["BASE_IMAGE"]; !ok {
					merged["BASE_IMAGE"] = img.Base
				}
			}
			args := make([]string, 0, len(merged))
			for _, k := range slices.Sorted(maps.Keys(merged)) {
				args = append(args, strutil.QuoteShell(k+"="+expandVars(resolveRefs(merged[k], tags), lock.Platform(platform))))
			}
			line := fmt.Sprintf("image_build %s %s %s", strutil.QuoteShell(st.Image), strutil.QuoteShell(img.Dockerfile), strutil.QuoteShell(tag))
			if len(args) > 0 {
				line += " " + strings.Join(args, " ")
			}
			b.WriteString(line + "\n")
			continue
		}
		c := p.Commands[st.Command]
		envs := make([]string, 0, len(c.Env))
		for _, k := range slices.Sorted(maps.Keys(c.Env)) {
			envs = append(envs, strutil.QuoteShell(k+"="+expandVars(resolveRefs(c.Env[k], tags), lock.Platform(platform))))
		}
		line := fmt.Sprintf("run_command %s %s %s", strutil.QuoteShell(st.Command), strutil.QuoteShell(c.Run), strutil.QuoteShell(mustDefault(c.Workdir, p.Project.Dir)))
		if len(envs) > 0 {
			line += " " + strings.Join(envs, " ")
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// expandVars 处理平台相关占位符：${platform} → linux-amd64，${arch} → amd64。
func expandVars(s string, platform lock.Platform) string {
	s = strings.ReplaceAll(s, "${platform}", string(platform))
	return strings.ReplaceAll(s, "${arch}", strings.TrimPrefix(string(platform), "linux-"))
}

// repoRootOverride 由 cmd 注入（--repo / CNB_REPO_ROOT），避免写死路径。
var repoRootOverride string

// SetRepoRoot 设置仓库根，供配置中的相对路径解析。
func SetRepoRoot(dir string) {
	if dir != "" {
		repoRootOverride = dir
	}
}

// repoRoot 返回仓库根：优先注入值，否则由可执行文件位置推导。
func repoRoot() string {
	if repoRootOverride != "" {
		return repoRootOverride
	}
	if root, err := workspace.FindFromExecutable(); err == nil {
		return root
	}
	if root, err := workspace.FindFromWorkingDir(); err == nil {
		return root
	}
	return "."
}

// Refs 返回当前平台需要的物料引用（id 或 id@version）。
func (p *File) Refs(platform string) []string {
	var out []string
	for _, id := range slices.Sorted(maps.Keys(p.Assets)) {
		a := p.Assets[id]
		if len(a.Platforms) > 0 && !containsStr(a.Platforms, platform) {
			continue
		}
		out = append(out, a.ref(id))
	}
	return out
}

// Render 校验并渲染为可执行 shell 片段。
func (p *File) Render(platform string, l *lock.File) (string, error) {
	if errs := p.Validate(platform, l); len(errs) > 0 {
		return "", fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return p.render(l, platform), nil
}

// detectRuntime 探测可用的容器运行时（podman 优先，其次 docker）。
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

func containsStr(list []string, v string) bool {
	return slices.Contains(list, v)
}

func resolveRefs(s string, tags map[string]string) string {
	return tagRefRe.ReplaceAllStringFunc(s, func(m string) string {
		name := tagRefRe.FindStringSubmatch(m)[1]
		if v, ok := tags[name]; ok {
			return v
		}
		return m
	})
}

// tagRefNames 返回字符串里引用的镜像名（${image.<name>.tag}）。
func tagRefNames(s string) []string {
	var out []string
	for _, m := range tagRefRe.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

func tagRefNamesInMap(m map[string]string) []string {
	var out []string
	for _, v := range m {
		out = append(out, tagRefNames(v)...)
	}
	return out
}

// versionVar 把物料 id 转成注入用的构建参数名，如 lifecycle → LIFECYCLE_VERSION。
func versionVar(id string) string {
	return strings.ToUpper(strings.ReplaceAll(id, "-", "_")) + "_VERSION"
}

func mustDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
