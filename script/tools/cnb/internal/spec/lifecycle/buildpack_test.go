package lifecycle

import "testing"

const bpSample = `
api = "0.12"

[buildpack]
  id = "paketo-buildpacks/go-build"
  name = "Go Build"
  version = "2.3.0"

[[metadata.dependencies]]
  name = "go"
  version = "1.27.4"

[[stacks]]
  id = "*"
`

func TestParseBuildpack(t *testing.T) {
	bp, warns, err := ParseBuildpack([]byte(bpSample))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("不应有告警: %v", warns)
	}
	if bp.API != "0.12" || bp.BuildpackInfo.ID != "paketo-buildpacks/go-build" || bp.BuildpackInfo.Version != "2.3.0" {
		t.Fatalf("字段解析不符: %+v", bp)
	}
	if len(bp.MetadataSection.Dependencies) != 1 {
		t.Fatalf("依赖解析不符: %+v", bp.MetadataSection)
	}
}

func TestParseBuildpackUnknownKey(t *testing.T) {
	_, warns, err := ParseBuildpack([]byte("api = \"0.12\"\nweird = true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 || warns[0].Path != "weird" {
		t.Fatalf("应报告未知键 weird: %+v", warns)
	}
}
