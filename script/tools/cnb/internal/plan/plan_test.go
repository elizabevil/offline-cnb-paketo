package plan

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
	"learn-cnb-paketo/tools/cnb/internal/spec/profile"
)

const sample = `
apiVersion = "cnb/v1"
kind       = "Pipeline"

[metadata]
name = "demo"

[spec.project]
dir      = "project/demo"
platform = "linux-amd64"

[spec.assets.lifecycle]
version = "0.21.29"

[spec.assets.fnm]
version = "1.39.0"

[[spec.stages]]
name    = "base"
workdir = "."
run     = ["podman", "build", "--build-context", "bundle=${runtime.bundle}/${spec.project.platform}", "-t", "x-${derived.architecture}", "."]

[[spec.stages]]
name    = "builder"
workdir = "project/demo"
run     = ["podman", "build", "--build-arg", "LIFECYCLE_VERSION=${spec.assets.lifecycle.version}", "."]
env     = { MODE = "release", ZED = "1" }
timeout = "5m"

[[spec.stages]]
name            = "flaky"
workdir         = "."
run             = ["false"]
continueOnError = true
`

const lockSample = `
schema = 1

[storage]
root = "bundle"

[[asset]]
id      = "lifecycle"
version = "0.21.20"
default = true

  [asset.platform.linux-amd64]
  url    = "https://example.com/lifecycle.tgz"
  sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  size   = 1
`

func mustParse(t *testing.T) (*profile.File, *manifest.File) {
	t.Helper()
	f, _, err := profile.Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	lock, _, err := manifest.Parse([]byte(lockSample))
	if err != nil {
		t.Fatal(err)
	}
	return f, lock
}

func TestBuildExpandsPlaceholders(t *testing.T) {
	f, lock := mustParse(t)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "project/demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(repo, "bundle")
	p, warns, err := Build(f, lock, Options{Repo: repo, Bundle: bundle})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("不应有告警: %v", warns)
	}
	base := p.Stages[0]
	wantCtx := "bundle=" + bundle + "/linux-amd64"
	if base.Argv[3] != wantCtx {
		t.Fatalf("bundle 展开错误: %q", base.Argv[3])
	}
	if base.Argv[5] != "x-amd64" {
		t.Fatalf("arch 展开错误: %q", base.Argv[5])
	}
	if base.Workdir != repo {
		t.Fatalf("workdir 未绝对化: %q", base.Workdir)
	}
	if base.Timeout != DefaultTimeout {
		t.Fatalf("默认 timeout 错误: %v", base.Timeout)
	}

	builder := p.Stages[1]
	if builder.Argv[3] != "LIFECYCLE_VERSION=0.21.29" {
		t.Fatalf("asset 版本展开错误: %q", builder.Argv[3])
	}
	if builder.Workdir != filepath.Join(repo, "project/demo") {
		t.Fatalf("workdir 错误: %q", builder.Workdir)
	}
	if builder.Timeout != 5*time.Minute {
		t.Fatalf("timeout 解析错误: %v", builder.Timeout)
	}
	if len(builder.Env) != 2 || builder.Env[0] != "MODE=release" || builder.Env[1] != "ZED=1" {
		t.Fatalf("env 未排序或解析错误: %v", builder.Env)
	}
	if !p.Stages[2].ContinueOnError {
		t.Fatal("continueOnError 未生效")
	}
}

func TestBuildUnknownPlaceholder(t *testing.T) {
	f, lock := mustParse(t)
	f.Spec.Stages[0].Run = []string{"echo", "${nope}"}
	if _, _, err := Build(f, lock, Options{Repo: t.TempDir()}); err == nil {
		t.Fatal("未知占位符应报错")
	}
}

func TestBuildAssetDefaultsToLock(t *testing.T) {
	f, lock := mustParse(t)
	delete(f.Spec.Assets, "lifecycle")
	// 模板引用已删除的 asset → 未知占位符
	if _, _, err := Build(f, lock, Options{Repo: t.TempDir()}); err == nil {
		t.Fatal("引用未声明物料应报错")
	}
}

func TestBuildInvalidPlatform(t *testing.T) {
	f, lock := mustParse(t)
	f.Spec.Project.Platform = "linux-mips"
	if _, _, err := Build(f, lock, Options{Repo: t.TempDir()}); err == nil {
		t.Fatal("非法平台应报错")
	}
}
