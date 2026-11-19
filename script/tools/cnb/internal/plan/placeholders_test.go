package plan

import (
	"strings"
	"testing"

	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
	"learn-cnb-paketo/tools/cnb/internal/spec/profile"
)

func TestPlaceholdersDocumented(t *testing.T) {
	f := &profile.File{}
	f.Spec.Project.Platform = "linux-arm64"
	f.Spec.Project.Base = map[string]string{"linux-arm64": "debian:slim"}
	f.Spec.Assets = map[string]profile.Asset{"cnb": {Version: "3.0.0"}}
	f.Spec.Stages = []profile.Stage{{Name: "x", Run: []string{"true"}}}

	rows, err := Placeholders(f, &manifest.File{}, Options{Repo: "/repo", Bundle: "/repo/bundle"})
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, r := range rows {
		values[r.Name] = r.Value
		if r.Source == "" {
			t.Errorf("placeholder %q has no source", r.Name)
		}
	}
	want := map[string]string{
		"spec.project.platform":      "linux-arm64",
		"derived.architecture":       "arm64",
		"derived.container_platform": "linux/arm64",
		"spec.project.base":          "debian:slim",
		"runtime.bundle":             "/repo/bundle",
		"runtime.repo":               "/repo",
		"spec.assets.cnb.version":    "3.0.0",
	}
	for name, value := range want {
		if values[name] != value {
			t.Errorf("${%s} = %q, want %q", name, values[name], value)
		}
	}
	for _, name := range []string{"spec.project.platform", "derived.architecture"} {
		if strings.Contains(name, "$") {
			t.Fatalf("unexpected name format: %s", name)
		}
	}
}
