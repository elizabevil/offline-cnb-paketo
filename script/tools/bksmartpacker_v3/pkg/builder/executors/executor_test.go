package executors

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"bksmartpacker/pkg/container"
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
		Socket: filepath.Join(t.TempDir(), "missing.sock"),
	}}
	if err := e.ensureRuntime(); err == nil {
		t.Fatal("expected missing managed socket error")
	}
}

func TestEnsureRuntimeHostIsNoop(t *testing.T) {
	e := &ContainerExecutor{runtime: container.Config{Driver: container.DriverHost}}
	if err := e.ensureRuntime(); err != nil {
		t.Fatal(err)
	}
}
