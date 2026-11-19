package profile

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Container CLI defaults for generated profiles.
const (
	ContainerPodman = "podman"
	ContainerDocker = "docker"
)

// DefaultBaseImage is only a starting point for new profiles. The base image
// decides what every stage is built on, so callers are expected to pass a real
// (usually registry-mirrored) reference.
const DefaultBaseImage = "debian:trixie-slim"

// TemplateData is the data available to profile.toml.tmpl.
type TemplateData struct {
	Name      string
	Dir       string
	Platform  string
	BaseImage string
	Container string
}

// Template renders the project profile template.
func Template(name, dir, platform, baseImage, container string) (string, error) {
	if baseImage == "" {
		baseImage = DefaultBaseImage
	}
	if platform == "" {
		platform = manifest.PlatformAMD64
	}
	switch container {
	case "":
		container = ContainerPodman
	case ContainerPodman, ContainerDocker:
	default:
		return "", fmt.Errorf("unsupported container %q: want podman | docker", container)
	}
	data := TemplateData{Name: name, Dir: dir, Platform: platform, BaseImage: baseImage, Container: container}
	return renderTemplate("templates/profile.toml.tmpl", data)
}

// renderTemplate renders one template from templates/.
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
