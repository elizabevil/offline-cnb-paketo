package manifest

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// DefaultPlatforms are the platforms a new manifest declares.
var DefaultPlatforms = []string{PlatformAMD64, PlatformARM64}

// LifecycleURL returns the release URL of the CNB lifecycle for one platform.
// Release assets are arch-specific, so every platform needs its own URL.
func LifecycleURL(platform string) string {
	arch := "x86-64"
	if ArchOf(Platform(platform)) == ArchARM64 {
		arch = "arm64"
	}
	return "https://github.com/buildpacks/lifecycle/releases/download/v{version}/lifecycle-v{version}+linux." + arch + ".tgz"
}

// PlatformAsset is one platform entry rendered into the template.
type PlatformAsset struct {
	Name string
	URL  string
}

// TemplateData is the data available to assets.toml.tmpl.
type TemplateData struct {
	Name      string
	Platforms []PlatformAsset
}

// Template renders a starter assets.toml for the given platforms.
func Template(name string, platforms []string) (string, error) {
	if len(platforms) == 0 {
		platforms = DefaultPlatforms
	}
	data := TemplateData{Name: name}
	for _, p := range platforms {
		data.Platforms = append(data.Platforms, PlatformAsset{Name: p, URL: LifecycleURL(p)})
	}
	raw, err := templateFS.ReadFile("templates/assets.toml.tmpl")
	if err != nil {
		return "", fmt.Errorf("read assets template: %w", err)
	}
	tpl, err := template.New("assets").Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse assets template: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render assets template: %w", err)
	}
	return buf.String(), nil
}
