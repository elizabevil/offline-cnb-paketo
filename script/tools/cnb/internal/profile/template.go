package profile

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// 支持的 Linux 发行版（决定包管理器与默认 base 镜像）。
const (
	DistroDebian = "debian"
	DistroUbuntu = "ubuntu"
	DistroAlpine = "alpine"
)

// distroDefaults 返回发行版的默认 base 镜像、包管理器与 stack id。
func distroDefaults(distro string) (base, pkgManager, stackID string) {
	switch distro {
	case DistroAlpine:
		return "alpine:3.20", "apk", "io.buildpacks.stacks.alpine"
	case DistroUbuntu:
		return "ubuntu:24.04", "apt", "io.buildpacks.stacks.jammy"
	default:
		return "debian:trixie-slim", "apt", "io.buildpacks.stacks.bionic"
	}
}

// TemplateData 是模板渲染所需的数据。
type TemplateData struct {
	Name       string
	Dir        string
	Platform   string
	Distro     string
	BaseImage  string
	PkgManager string
	StackID    string
}

// Template 渲染项目配置模板（内容在 templates/profile.toml.tmpl）。
func Template(name, dir, platform, distro string) (string, error) {
	if distro == "" {
		distro = DistroDebian
	}
	base, pkgMgr, stackID := distroDefaults(distro)
	data := TemplateData{
		Name: name, Dir: dir, Platform: platform,
		Distro: distro, BaseImage: base, PkgManager: pkgMgr, StackID: stackID,
	}
	return renderTemplate("templates/profile.toml.tmpl", data)
}

// renderTemplate 渲染 templates/ 下的模板。
func renderTemplate(name string, data any) (string, error) {
	raw, err := templateFS.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", name, err)
	}
	tpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template %s: %w", name, err)
	}
	return buf.String(), nil
}
