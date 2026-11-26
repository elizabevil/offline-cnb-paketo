package builder

import (
	"encoding/json"
	"testing"

	"bksmartpacker/pkg/types"
)

func TestGenerateAppProcfile(t *testing.T) {
	modules := []types.ModuleDescription{
		{Name: "default", Spec: types.BkAppSpec{Processes: []types.Process{
			{Name: "web", ProcCommand: "bin/start_web.sh"},
			{Name: "beat", ProcCommand: "python manage.py celery beat"},
		}}},
		{Name: "pipeline", Spec: types.BkAppSpec{Processes: []types.Process{
			{Name: "web", ProcCommand: "bin/start_exporter.sh"},
			{Name: "node-timeout", ProcCommand: "python manage.py node_timeout_process"},
		}}},
	}
	data, err := GenerateAppProcfile(modules)
	if err != nil {
		t.Fatal(err)
	}
	var procs map[string]string
	if err := yamlUnmarshal(data, &procs); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"default-web", "default-beat", "pipeline-web", "pipeline-node-timeout"} {
		if _, ok := procs[key]; !ok {
			t.Fatalf("missing %s in %v", key, procs)
		}
	}
	if procs["default-web"] == procs["pipeline-web"] {
		t.Fatal("same-name web should be namespaced")
	}
}

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
	if got := artifact.AppArtifacts["pipeline"].ProcEntrypoints["node-timeout"]; len(got) != 1 || got[0] != "pipeline-node-timeout" {
		t.Fatalf("unexpected entrypoint: %v", got)
	}
}
