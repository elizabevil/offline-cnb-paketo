package plan

import (
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
