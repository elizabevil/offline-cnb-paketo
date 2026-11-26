package smartpkg

import "testing"

func TestIsLayerArchive(t *testing.T) {
	for _, name := range []string{"default.tgz", "default.tar"} {
		if !isLayerArchive(name) {
			t.Fatalf("%s should be a layer archive", name)
		}
	}
	for _, name := range []string{"default.zip", "app_desc.yaml", "artifact.json"} {
		if isLayerArchive(name) {
			t.Fatalf("%s should not be a layer archive", name)
		}
	}
}

func TestIsSourcePath(t *testing.T) {
	cases := map[string]bool{
		"app/requirements.txt": true,
		"app/package.json":     true,
		"app/foo.csproj":       true,
		"workspace/go.mod":     true,
		"app/Procfile":         false,
		"app/src/main.go":      false,
		"layers/paketo-buildpacks_pip-install/site-packages/setup.py": false,
		"app/site-packages/setup.py":                                  false,
		"app/vendor/package.json":                                     false,
	}
	for name, want := range cases {
		if got := isSourcePath(name); got != want {
			t.Errorf("isSourcePath(%q)=%v, want %v", name, got, want)
		}
	}
}
