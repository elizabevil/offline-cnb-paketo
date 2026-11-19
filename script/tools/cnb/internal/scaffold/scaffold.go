// Package scaffold renders the files a new project starts with.
//
// The template list is data: adding or removing a generated file means editing
// Templates(), not the command. Callers can skip entries (--skip) or render a
// subset (--only).
package scaffold

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/lock"
	specprofile "learn-cnb-paketo/tools/cnb/internal/spec/profile"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Options are the inputs shared by all templates.
type Options struct {
	Name      string
	Dir       string
	Platform  string
	Platforms []string
	BaseImage string
	Container string
}

// File is one rendered output.
type File struct {
	// ID is the template id (config, assets, dockerfile-base, …).
	ID      string
	Path    string
	Content string
}

// Template describes one generated file.
type Template struct {
	ID          string
	Description string
	Path        func(Options) string
	Render      func(Options) (string, error)
}

// Templates returns the scaffold definition in write order.
func Templates() []Template {
	return []Template{
		{
			ID:          "config",
			Description: "pipeline config (stages, placeholders, checks)",
			Path:        func(o Options) string { return filepath.Join(o.Dir, "pipeline.toml") },
			Render: func(o Options) (string, error) {
				return specprofile.Template(o.Name, o.Dir, o.Platform, o.BaseImage, o.Container)
			},
		},
		{
			ID:          "assets",
			Description: "material manifest (assets.toml)",
			Path:        func(o Options) string { return filepath.Join(o.Dir, "assets.toml") },
			Render: func(o Options) (string, error) {
				return lock.Template(o.Name, o.Platforms)
			},
		},
		{
			ID:          "justfile",
			Description: "project task runner (just)",
			Path:        func(o Options) string { return filepath.Join(o.Dir, "justfile") },
			Render:      func(o Options) (string, error) { return renderText("justfile", o.Name) },
		},
		{
			ID:          "readme",
			Description: "project README",
			Path:        func(o Options) string { return filepath.Join(o.Dir, "README.md") },
			Render:      func(o Options) (string, error) { return renderText("README.md", o.Name) },
		},
		{
			ID:          "dockerfile-base",
			Description: "base image Dockerfile",
			Path:        func(o Options) string { return filepath.Join(o.Dir, "docker", "base.dockerfile") },
			Render:      func(o Options) (string, error) { return renderText("base.dockerfile", o.Name) },
		},
		{
			ID:          "dockerfile-builder",
			Description: "builder image Dockerfile",
			Path:        func(o Options) string { return filepath.Join(o.Dir, "docker", "builder.dockerfile") },
			Render:      func(o Options) (string, error) { return renderText("builder.dockerfile", o.Name) },
		},
	}
}

// renderText renders templates/<kind>.tmpl with the project name substituted.
// These templates contain literal just syntax ({{...}}), so only the project
// name placeholder is replaced.
func renderText(kind, name string) (string, error) {
	raw, err := templateFS.ReadFile("templates/" + kind + ".tmpl")
	if err != nil {
		return "", fmt.Errorf("read %s template: %w", kind, err)
	}
	return strings.ReplaceAll(string(raw), "{{.Name}}", name), nil
}

// Render produces every selected file. IDs in only/skip filter the list;
// an empty only means "all".
func Render(o Options, only, skip []string) ([]File, error) {
	onlySet := toSet(only)
	skipSet := toSet(skip)
	var out []File
	for _, t := range Templates() {
		if len(onlySet) > 0 && !onlySet[t.ID] {
			continue
		}
		if skipSet[t.ID] {
			continue
		}
		content, err := t.Render(o)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", t.ID, err)
		}
		out = append(out, File{ID: t.ID, Path: t.Path(o), Content: content})
	}
	return out, nil
}

func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]bool, len(items))
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			set[item] = true
		}
	}
	return set
}
