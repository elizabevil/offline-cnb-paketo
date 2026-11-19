package plan

import (
	"path/filepath"
	"testing"

	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
	"learn-cnb-paketo/tools/cnb/internal/spec/profile"
)

func TestExpandChecksUsesSameVarsAsStages(t *testing.T) {
	repo := t.TempDir()
	platform := manifest.Platform("linux-amd64")
	f := &profile.File{}
	f.Spec.Project.Directory = "project/demo"
	f.Spec.Project.Platform = string(platform)
	f.Spec.Checks.Dockerfiles = []profile.DockerfileCheck{{
		Path:     "docker/app.dockerfile",
		Context:  ".",
		Contexts: map[string]string{"bundle": "${runtime.bundle}", "buildpacks": "${runtime.repo}/buildpacks/paketo"},
	}}
	p, _, err := Build(f, &manifest.File{}, Options{Repo: repo, Bundle: filepath.Join(repo, "bundle")})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Dockerfiles) != 1 {
		t.Fatalf("want 1 target, got %d", len(p.Dockerfiles))
	}
	got := p.Dockerfiles[0]
	if got.Dockerfile != filepath.Join(repo, "project/demo/docker/app.dockerfile") {
		t.Errorf("dockerfile = %s", got.Dockerfile)
	}
	if got.Context != filepath.Join(repo, "project/demo") {
		t.Errorf("context = %s", got.Context)
	}
	if got.Contexts["bundle"] != filepath.Join(repo, "bundle") {
		t.Errorf("bundle = %s", got.Contexts["bundle"])
	}
	if got.Contexts["buildpacks"] != filepath.Join(repo, "buildpacks/paketo") {
		t.Errorf("buildpacks = %s", got.Contexts["buildpacks"])
	}
}
