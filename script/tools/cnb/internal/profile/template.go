package profile

import (
	"fmt"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// Template 生成一份可直接使用的新项目配置模板。
// 只写结构，不猜业务：物料、镜像、命令都留成带注释的占位。
func Template(name, dir, platform string) string {
	return fmt.Sprintf(`# ─── %s 流水线 ────────────────────────────────────────────────────────────
# 改版本 → [assets.*]；改镜像 → [images.*]；改流程顺序 → after
# 所有路径都相对仓库根；校验：cnb profile validate config/projects/%s.toml

schema = 1

[project]
name     = "%s"
dir      = "%s"          # 项目目录（相对仓库根）
platform = "%s"

[docker]
context       = "."             # docker build 上下文（相对仓库根）
bundle_layout = "arch-first"    # bundle/<platform>/{archives,binaries}

# ── 物料：键名即 id，version 省略则取 lock 的 default ───────────────────────
[assets.lifecycle]
version = "0.21.13"

# [assets.fnm]
# version = "1.39.0"

# ── 镜像：键名即阶段名，after 决定顺序 ──────────────────────────────────────
[images.base]
dockerfile = "%s/docker/Dockerfile.base"
tag        = "%s-builder-base:latest"
args       = { BASE_IMAGE = "debian:trixie-slim" }

# [images.builder]
# dockerfile = "%s/docker/Dockerfile"
# tag        = "%s-builder:latest"
# after      = ["base"]
# args       = {}

# ── 命令：构建完成后执行（打包、测试等），workdir 默认 = project.dir ─────────
# [commands.package]
# run     = "examples/build.sh"
# workdir = "%s"

# ── 流程：顺序即数组顺序 ────────────────────────────────────────────────────
[[stage]]
image = "base"

# [[stage]]
# command = "package"
`, name, name, name, dir, platform, dir, name, dir, name, dir)
}

// Show 生成分组视图，避免平铺。
func (p *File) Show() string {
	var b []byte
	add := func(format string, args ...any) { b = append(b, []byte(fmt.Sprintf(format, args...))...) }

	add("项目\n")
	add("  name      %s\n", p.Project.Name)
	add("  dir       %s\n", p.Project.Dir)
	add("  platform  %s\n", p.Project.Platform)

	add("\n构建\n")
	add("  context       %s\n", mustDefault(p.Docker.Context, "."))
	add("  bundle_layout %s\n", mustDefault(p.Docker.BundleLayout, lock.LayoutArchFirst))
	if p.Docker.BundleDir != "" {
		add("  bundle_dir    %s\n", p.Docker.BundleDir)
	}

	add("\n物料（%d）\n", len(p.Assets))
	for _, id := range sortedKeys(p.Assets) {
		a := p.Assets[id]
		ver := mustDefault(a.Version, "(default)")
		if len(a.Platforms) > 0 {
			add("  %-12s %s  仅 %v\n", id, ver, a.Platforms)
		} else {
			add("  %-12s %s\n", id, ver)
		}
	}

	add("\n镜像（%d）\n", len(p.Images))
	for _, name := range sortedKeys(p.Images) {
		img := p.Images[name]
		if img.Desc != "" {
			add("  %-12s %s\n", name, img.Desc)
			add("  %-12s   %s → %s\n", "", img.Dockerfile, img.Tag)
		} else {
			add("  %-12s %s → %s\n", name, img.Dockerfile, img.Tag)
		}
	}

	add("\n命令（%d）\n", len(p.Commands))
	for _, name := range sortedKeys(p.Commands) {
		c := p.Commands[name]
		add("  %-12s cd %s && %s\n", name, mustDefault(c.Workdir, p.Project.Dir), c.Run)
	}

	add("\n流程（%d 步）\n", len(p.Stages))
	for i, st := range p.Stages {
		kind, name := "image", st.Image
		if st.Command != "" {
			kind, name = "command", st.Command
		}
		add("  %2d. %-8s %s\n", i+1, kind, name)
	}
	return string(b)
}
