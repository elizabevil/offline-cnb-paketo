package scaffold

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// fixture builds a fake reference project with two files and one nested dir.
func fixture(t *testing.T) string {
	t.Helper()
	ref := filepath.Join(t.TempDir(), "ref")
	files := map[string]string{
		"pipeline.toml":           "name = \"ref\"\n",
		"docker/base.dockerfile":  "FROM scratch\n",
		"docker/apt/sources.list": "deb x\n",
		"docs/architecture.md":    "# ref\n",
	}
	for rel, content := range files {
		p := filepath.Join(ref, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ref
}

func paths(files []File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, filepath.ToSlash(f.Path))
	}
	slices.Sort(out)
	return out
}

func TestRenderOnly(t *testing.T) {
	ref := fixture(t)
	got, err := Render(Options{Name: "new", Dir: "project/new", From: ref, Only: []string{"docker"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docker/apt/sources.list", "docker/base.dockerfile"}
	if !slices.Equal(paths(got), want) {
		t.Errorf("got %v, want %v", paths(got), want)
	}
}

func TestRenderSkip(t *testing.T) {
	ref := fixture(t)
	got, err := Render(Options{Name: "new", Dir: "project/new", From: ref, Skip: []string{"docker", "docs"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"pipeline.toml"}; !slices.Equal(paths(got), want) {
		t.Errorf("got %v, want %v", paths(got), want)
	}
}

func TestRenderSkipAssets(t *testing.T) {
	ref := fixture(t)
	assets := filepath.Join(t.TempDir(), "assets.toml")
	if err := os.WriteFile(assets, []byte("schema = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Render(Options{Name: "new", Dir: "project/new", From: ref, Assets: assets, Skip: []string{"assets.toml"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths(got) {
		if p == "assets.toml" {
			t.Fatalf("assets.toml should be skipped: %v", paths(got))
		}
	}
}

func TestRenderSkipGlob(t *testing.T) {
	ref := fixture(t)
	got, err := Render(Options{Name: "new", Dir: "project/new", From: ref, Skip: []string{"docker/*.dockerfile"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths(got) {
		if p == "docker/base.dockerfile" {
			t.Fatalf("glob skip failed: %v", paths(got))
		}
	}
}

func TestRenderRewritesNameAndDir(t *testing.T) {
	ref := fixture(t)
	got, err := Render(Options{Name: "new", Dir: "project/new", From: ref})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		if f.Path == "pipeline.toml" && string(f.Content) != "name = \"new\"\n" {
			t.Errorf("content = %q", f.Content)
		}
	}
}
