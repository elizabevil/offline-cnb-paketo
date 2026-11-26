package container

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseRuntimeSelection(t *testing.T) {
	cases := []struct {
		runtime string
		backend Backend
		env     Environment
	}{
		{"", "", EnvironmentHost},
		{runtimeNamePind, BackendPodman, EnvironmentPind},
		{runtimeNameDind, BackendDocker, EnvironmentDind},
		{runtimeNamePodman, BackendPodman, EnvironmentHost},
		{runtimeNameDocker, BackendDocker, EnvironmentHost},
		{runtimeNameHostPodman, BackendPodman, EnvironmentHost},
		{runtimeNameHostDocker, BackendDocker, EnvironmentHost},
	}
	for _, tc := range cases {
		selection, err := parseRuntimeSelection(tc.runtime)
		if err != nil || selection.Backend != tc.backend || selection.Environment != tc.env {
			t.Fatalf("%s: selection=%#v err=%v", tc.runtime, selection, err)
		}
	}
	if _, err := parseRuntimeSelection("bad"); err == nil {
		t.Fatal("expected invalid runtime error")
	}
}

func TestResolveOuterBackendIndependent(t *testing.T) {
	selection := runtimeSelection{Environment: EnvironmentPind, Backend: BackendPodman}
	if backend, err := resolveBackend(selection, "podman"); err != nil || backend != BackendPodman {
		t.Fatalf("backend=%s err=%v", backend, err)
	}
	// outer docker + inner podman 是允许的，外层不再被 inner backend 绑定。
	if backend, err := resolveBackend(selection, "docker"); err != nil || backend != BackendDocker {
		t.Fatalf("backend=%s err=%v", backend, err)
	}
}

func TestValidatePair(t *testing.T) {
	if err := validatePair(BackendPodman, EnvironmentPind); err != nil {
		t.Fatal(err)
	}
	if err := validatePair(BackendDocker, EnvironmentDind); err != nil {
		t.Fatal(err)
	}
	if err := validatePair(BackendDocker, EnvironmentPind); err == nil {
		t.Fatal("docker+pind should fail")
	}
}

func TestResolveExportMode(t *testing.T) {
	if mode, err := resolveExportMode(BackendPodman, EnvironmentPind, ""); err != nil || mode != ExportLayout {
		t.Fatalf("pind mode=%s err=%v", mode, err)
	}
	if mode, err := resolveExportMode(BackendDocker, EnvironmentDind, ""); err != nil || mode != ExportDaemon {
		t.Fatalf("dind mode=%s err=%v", mode, err)
	}
	if _, err := resolveExportMode(BackendPodman, EnvironmentHost, "daemon"); err == nil {
		t.Fatal("podman daemon should fail")
	}
}

func TestResolveSocketExplicit(t *testing.T) {
	if got := resolveSocket(BackendPodman, EnvironmentHost, "/tmp/podman.sock", ""); got != "/tmp/podman.sock" {
		t.Fatalf("socket = %q", got)
	}
}

func TestResolveCache(t *testing.T) {
	dir := t.TempDir()
	cache, err := resolveCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cache.HostPath != dir || cache.ContainerPath != DefaultCacheContainerPath {
		t.Fatalf("cache = %#v", cache)
	}
}

func TestResolveCacheDisabled(t *testing.T) {
	cache, err := resolveCache("")
	if err != nil {
		t.Fatal(err)
	}
	if cache.Enabled() {
		t.Fatalf("cache should be disabled: %#v", cache)
	}
}

func TestResolveDriver(t *testing.T) {
	driver, err := resolveDriver("")
	if err != nil || driver != DriverHost {
		t.Fatalf("driver=%s err=%v", driver, err)
	}
	driver, err = resolveDriver("managed")
	if err != nil || driver != DriverManaged {
		t.Fatalf("driver=%s err=%v", driver, err)
	}
	if _, err := resolveDriver("bad"); err == nil {
		t.Fatal("expected invalid driver error")
	}
}

func TestParseWithInjectedEnv(t *testing.T) {
	cacheDir := t.TempDir()
	env := map[string]string{
		"BKS_RUNTIME":      "host-podman",
		"CNB_CACHE_DIR":    cacheDir,
		"BKS_PLATFORM":     "linux/amd64",
		"BKS_NETWORK":      "host",
		"BKS_EXTRA_MOUNTS": "--mount type=bind,source=/tmp,target=/tmp",
	}
	cfg, err := Parse(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Outer.Backend != BackendPodman || cfg.Inner.Backend != BackendPodman || cfg.Environment != EnvironmentHost {
		t.Fatalf("cfg = %#v", cfg)
	}
	if cfg.Cache.HostPath != cacheDir || cfg.Platform != "linux/amd64" || cfg.Network != "host" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestParseManagedRequiresWorkspace(t *testing.T) {
	env := map[string]string{
		"BKS_RUNTIME":        "pind",
		"CNB_RUNTIME_DRIVER": "managed",
		"BKS_CONTAINER_CLI":  "podman",
	}
	if _, err := Parse(func(key string) string { return env[key] }); err == nil {
		t.Fatal("expected managed workspace error")
	}
}

func TestResolveSidecarTimeout(t *testing.T) {
	if timeout, err := resolveSidecarTimeout(""); err != nil || timeout != DefaultSidecarTimeout {
		t.Fatalf("timeout=%s err=%v", timeout, err)
	}
	if timeout, err := resolveSidecarTimeout("90s"); err != nil || timeout != 90*time.Second {
		t.Fatalf("timeout=%s err=%v", timeout, err)
	}
	if _, err := resolveSidecarTimeout("bad"); err == nil {
		t.Fatal("expected invalid timeout error")
	}
}

func TestParseDind(t *testing.T) {
	dir := t.TempDir()
	docker := filepath.Join(dir, "docker")
	if err := os.WriteFile(docker, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"RUNTIME":           "dind",
		"BKS_CONTAINER_CLI": docker,
	}
	cfg, err := Parse(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Outer.Backend != BackendDocker || cfg.Inner.Backend != BackendDocker || cfg.Environment != EnvironmentDind || cfg.ExportMode != ExportDaemon {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestParseOuterPodmanInnerDocker(t *testing.T) {
	dir := t.TempDir()
	docker := filepath.Join(dir, "docker")
	if err := os.WriteFile(docker, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"RUNTIME":                 "dind",
		"CNB_RUNTIME_DRIVER":      "managed",
		"CNB_RUNTIME_WORKSPACE":   filepath.Join(dir, "workspace"),
		"CNB_RUNTIME_IMAGE":       "runtime-dind:test",
		"BKS_CONTAINER_CLI":       "podman",
		"BKS_INNER_CONTAINER_CLI": docker,
	}
	cfg, err := Parse(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Outer.Backend != BackendPodman || cfg.Inner.Backend != BackendDocker {
		t.Fatalf("outer=%s inner=%s", cfg.Outer.Backend, cfg.Inner.Backend)
	}
	if cfg.ExportMode != ExportDaemon {
		t.Fatalf("export=%s", cfg.ExportMode)
	}
}
