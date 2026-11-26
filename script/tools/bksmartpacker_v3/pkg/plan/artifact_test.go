package plan

import (
	"encoding/json"
	"testing"

	"bksmartpacker/pkg/types"
)

func TestBuildArtifactJSON(t *testing.T) {
	plan := &BuildPlan{
		BaseImageID:  "default",
		Architecture: "amd64",
		Groups: []*ModuleBuildGroup{
			{ModuleName: "default", ModuleNames: []string{"default", "pipeline"}, OutputImageTarName: "default.tar"},
		},
		ModuleProcesses: map[string][]ModuleProcess{
			"default":  {{Name: "web", Command: "bin/start_web.sh"}},
			"pipeline": {{Name: "node-timeout", Command: "python manage.py node_timeout_process"}},
		},
	}
	data, err := BuildArtifactJSON(plan)
	if err != nil {
		t.Fatal(err)
	}
	var artifact types.ArtifactJSON
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Runtime.BaseImageID != "default" || artifact.Runtime.Architecture != "amd64" {
		t.Fatalf("unexpected runtime: %+v", artifact.Runtime)
	}
	if artifact.AppArtifacts["pipeline"].ImageTar != "default.tar" {
		t.Fatalf("unexpected image_tar: %+v", artifact.AppArtifacts["pipeline"])
	}
	if got := artifact.AppArtifacts["pipeline"].ProcEntrypoints["node-timeout"]; len(got) != 1 || got[0] != "/cnb/process/pipeline-node-timeout" {
		t.Fatalf("unexpected entrypoint: %v", got)
	}
}
