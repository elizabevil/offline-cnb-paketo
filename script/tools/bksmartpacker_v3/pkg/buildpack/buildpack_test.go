package buildpack

import (
	"reflect"
	"testing"

	"bksmartpacker/pkg/types"
)

func TestForModule(t *testing.T) {
	t.Setenv("BKS_APTFILE", "")
	module := types.ModuleDescription{Language: "Python"}
	got := ForModule(&module)
	want := []Buildpack{{Name: BuildpackPython}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default buildpack = %#v", got)
	}

	module.Spec.Build.Buildpacks = []types.Buildpack{{Name: "custom-python", Version: "v1"}}
	got = ForModule(&module)
	want = []Buildpack{{Name: "custom-python", Version: "v1"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configured buildpack = %#v", got)
	}
}
