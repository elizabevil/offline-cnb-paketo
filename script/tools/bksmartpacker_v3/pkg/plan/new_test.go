package plan

import (
	"testing"

	"bksmartpacker/pkg/types"
)

func TestNew(t *testing.T) {
	appDesc := types.ApplicationDescription{
		AppVersion: "1.0",
		AppInfo:    types.AppInfo{Code: "demo"},
		Modules: []types.ModuleDescription{{
			Name:      "web",
			Language:  "Python",
			SourceDir: ".",
			Spec: types.BkAppSpec{Processes: []types.Process{{
				Name:        "web",
				ProcCommand: "bin/start_web.sh",
			}}},
		}},
	}
	p, err := New(Input{
		AppDesc:    appDesc,
		RawAppDesc: "raw",
		Params:     &types.Params{Src: t.TempDir(), PackagingVersion: PackagingVersionV2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Steps) != 1 || len(p.Groups) != 1 {
		t.Fatalf("steps=%d groups=%d", len(p.Steps), len(p.Groups))
	}
	if p.Groups[0].OutputImageTarName != "web.tar" {
		t.Fatalf("image tar = %q", p.Groups[0].OutputImageTarName)
	}
}
