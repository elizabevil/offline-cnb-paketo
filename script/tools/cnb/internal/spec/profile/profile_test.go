package profile

import "testing"

const sample = `
apiVersion = "cnb/v1"
kind       = "Pipeline"

[metadata]
name = "offline-paketo"

[spec.project]
directory = "project/offline-paketo"
platform = "linux-amd64"

[spec.assets.lifecycle]
version = "0.21.29"

[[spec.stages]]
name    = "base"
workdir = "."
run     = ["podman", "build", "-t", "offline-builder-base:latest", "."]

[[spec.stages]]
name    = "package"
workdir = "project/offline-paketo"
run     = ["bash", "scripts/build.sh"]
timeout = "10m"

  [spec.stages.env]
  MODE = "release"
`

func TestParse(t *testing.T) {
	f, warns, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("不应有告警: %v", warns)
	}
	if f.Metadata.Name != "offline-paketo" || len(f.Spec.Stages) != 2 {
		t.Fatalf("解析结果不符: %+v", f)
	}
	if f.Spec.Stages[1].Env["MODE"] != "release" {
		t.Fatalf("env 解析失败: %+v", f.Spec.Stages[1])
	}
	if errsOut := f.Validate(); len(errsOut) != 0 {
		t.Fatalf("不应有校验错误: %v", errsOut)
	}
}

func TestParseUnknownKey(t *testing.T) {
	_, warns, err := Parse([]byte("apiVersion = \"cnb/v1\"\nfoo = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 || warns[0].Path != "foo" {
		t.Fatalf("应报告未知键: %+v", warns)
	}
}

func TestValidateErrors(t *testing.T) {
	f := &File{
		APIVersion: "v0",
		Kind:       "Deployment",
		Spec: Spec{
			Stages: []Stage{
				{Name: "a"},
				{Name: "a", Run: []string{"x"}, Workdir: ".", Timeout: "bad"},
			},
		},
	}
	if n := len(f.Validate()); n < 5 {
		t.Fatalf("应报告多条错误，实际 %d", n)
	}
}

func TestValidateDuplicateStage(t *testing.T) {
	f, _, _ := Parse([]byte(sample))
	f.Spec.Stages = append(f.Spec.Stages, f.Spec.Stages[0])
	errsOut := f.Validate()
	found := false
	for _, e := range errsOut {
		if e != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("重复 stage 应报错")
	}
}
