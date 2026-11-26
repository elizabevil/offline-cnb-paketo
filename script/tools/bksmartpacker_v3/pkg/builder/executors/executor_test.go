package executors

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"bksmartpacker/pkg/container"
	"bksmartpacker/pkg/plan"
)

func TestBuildArgsAddsCache(t *testing.T) {
	ws, err := NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.Close() }()
	e := &ContainerExecutor{runtime: container.Config{
		Cache: container.Cache{HostPath: "/tmp/cache", ContainerPath: "/layers/cache"},
	}, workspace: ws}
	args := e.buildArgs(buildInput{})
	if !contains(args, "type=bind,source=/tmp/cache,target=/layers/cache") {
		t.Fatalf("cache mount missing: %v", args)
	}
	if !contains(args, "CNB_CACHE_DIR=/layers/cache") {
		t.Fatalf("cache env missing: %v", args)
	}
}

func TestBuildArgsDoesNotDuplicateCacheMount(t *testing.T) {
	ws, err := NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.Close() }()
	e := &ContainerExecutor{runtime: container.Config{
		Cache:       container.Cache{HostPath: "/tmp/cache", ContainerPath: "/layers/cache"},
		ExtraMounts: []string{"--mount", "type=bind,source=/custom,target=/layers/cache"},
	}, workspace: ws}
	args := e.buildArgs(buildInput{})
	var count int
	for _, arg := range args {
		if strings.Contains(arg, "target=/layers/cache") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("cache mount count = %d, args=%v", count, args)
	}
}

func contains(values []string, want string) bool {
	return slices.Contains(values, want)
}

func TestEnsureRuntimeManagedRequiresSocket(t *testing.T) {
	e := &ContainerExecutor{runtime: container.Config{
		Driver: container.DriverManaged,
		Inner:  container.RuntimeConfig{Socket: filepath.Join(t.TempDir(), "missing.sock")},
	}}
	if err := e.ensureRuntime(context.Background()); err == nil {
		t.Fatal("expected missing managed socket error")
	}
}

func TestEnsureRuntimeHostIsNoop(t *testing.T) {
	e := &ContainerExecutor{runtime: container.Config{Driver: container.DriverHost}}
	if err := e.ensureRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestArgBuilder(t *testing.T) {
	a := newArgBuilder("run", "--rm")
	a.env("A", "1")
	a.bindMount("/host", "/container", true)
	a.option("--network", "host")
	got := a.args()
	want := []string{
		"run", "--rm",
		"-e", "A=1",
		"--mount", "type=bind,source=/host,target=/container,readonly",
		"--network", "host",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestImageSaveArgs(t *testing.T) {
	got := imageSaveArgs(container.RuntimeConfig{Backend: container.BackendPodman}, "/tmp/out.tar", "bk/app:1")
	want := []string{"image", "save", "--format", "oci-archive", "-o", "/tmp/out.tar", "bk/app:1"}
	if !slices.Equal(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestDockerArchiveSaveArgs(t *testing.T) {
	got := dockerArchiveSaveArgs("/tmp/run-image.tar", "fake-cnb-runner:latest")
	want := []string{"image", "save", "-o", "/tmp/run-image.tar", "fake-cnb-runner:latest"}
	if !slices.Equal(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestBuildContainerName(t *testing.T) {
	name := buildContainerName("app", "default")
	if !strings.HasPrefix(name, "cnb-build-app-default-") {
		t.Fatalf("name = %q", name)
	}
	if name == buildContainerName("app", "default") {
		t.Fatal("container names should be unique")
	}
}

func TestSidecarSpec(t *testing.T) {
	ws, err := NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.Close() }()
	e := &ContainerExecutor{
		plan:      plan.BuildPlan{AppCode: "app"},
		workspace: ws,
		runtime: container.Config{
			Inner:        container.RuntimeConfig{Backend: container.BackendPodman},
			RuntimeImage: "runtime-pind:test",
			Cache:        container.Cache{HostPath: "/tmp/cache", ContainerPath: "/layers/cache"},
			Network:      "host",
		},
	}
	spec := e.sidecarSpec("sidecar")
	if spec.Name != "sidecar" || spec.Image != "runtime-pind:test" || !spec.Privileged {
		t.Fatalf("sidecar spec = %#v", spec)
	}
	if len(spec.Env) != 1 || spec.Env[0].Name != container.EnvRuntimeCache || spec.Env[0].Value != "/cnb/runtime" {
		t.Fatalf("sidecar env = %#v", spec.Env)
	}
	hasWorkspace, hasVolume := false, false
	for _, mount := range spec.Mounts {
		if mount.Type == "bind" && mount.Source == ws.Host && mount.Target == ws.Container {
			hasWorkspace = true
		}
		if mount.Type == "volume" && mount.Source == "cnb-runtime-podman" && mount.Target == "/cnb/runtime" {
			hasVolume = true
		}
	}
	if !hasWorkspace || !hasVolume {
		t.Fatalf("sidecar mounts = %#v", spec.Mounts)
	}
	args := spec.args()
	if !slices.Contains(args, "runtime-pind:test") || !slices.Contains(args, "--privileged") {
		t.Fatalf("sidecar args = %#v", args)
	}
}

func TestRuntimeHealthy(t *testing.T) {
	script := filepath.Join(t.TempDir(), "runtime-health.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	e := &ContainerExecutor{runtime: container.Config{Inner: container.RuntimeConfig{Binary: script, Socket: "/tmp/any.sock"}}}
	if !e.runtimeHealthy(context.Background()) {
		t.Fatal("healthy runtime should pass")
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if e.runtimeHealthy(context.Background()) {
		t.Fatal("failed runtime should not be healthy")
	}
}

func TestPlatformNormalization(t *testing.T) {
	if got := containerPlatform("linux-amd64"); got != "linux/amd64" {
		t.Fatalf("containerPlatform = %q", got)
	}
	if osName, arch := splitPlatform("linux-arm64"); osName != "linux" || arch != "arm64" {
		t.Fatalf("splitPlatform = %s/%s", osName, arch)
	}
}
