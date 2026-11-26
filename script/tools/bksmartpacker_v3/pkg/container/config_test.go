package container

import (
	"reflect"
	"testing"
)

func TestSaveArgsPodman(t *testing.T) {
	got := (Config{Backend: BackendPodman}).SaveArgs("/tmp/web.tgz", "bk/web:1")
	want := []string{"image", "save", "--format", "oci-archive", "-o", "/tmp/web.tgz", "bk/web:1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("podman 参数错误: %v", got)
	}
}

func TestSaveArgsDocker(t *testing.T) {
	got := (Config{Backend: BackendDocker}).SaveArgs("/tmp/web.tgz", "bk/web:1")
	want := []string{"image", "save", "-o", "/tmp/web.tgz", "bk/web:1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("docker 参数错误: %v", got)
	}
}

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

func TestResolveBackend(t *testing.T) {
	selection := runtimeSelection{Environment: EnvironmentPind, Backend: BackendPodman}
	if backend, err := resolveBackend(selection, "podman"); err != nil || backend != BackendPodman {
		t.Fatalf("backend=%s err=%v", backend, err)
	}
	if _, err := resolveBackend(selection, "docker"); err == nil {
		t.Fatal("pind + docker should fail")
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
	t.Setenv("BKS_EXPORT_MODE", "")
	if mode, err := resolveExportMode(BackendPodman, EnvironmentPind); err != nil || mode != ExportLayout {
		t.Fatalf("pind mode=%s err=%v", mode, err)
	}
	if mode, err := resolveExportMode(BackendDocker, EnvironmentDind); err != nil || mode != ExportDaemon {
		t.Fatalf("dind mode=%s err=%v", mode, err)
	}
	t.Setenv("BKS_EXPORT_MODE", "daemon")
	if _, err := resolveExportMode(BackendPodman, EnvironmentHost); err == nil {
		t.Fatal("podman daemon should fail")
	}
}

func TestResolveSocketExplicit(t *testing.T) {
	t.Setenv("BKS_DOCKER_SOCK", "/tmp/podman.sock")
	if got := resolveSocket(BackendPodman, EnvironmentHost); got != "/tmp/podman.sock" {
		t.Fatalf("socket = %q", got)
	}
}

func TestResolveCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CNB_CACHE_DIR", dir)
	cache, err := resolveCache()
	if err != nil {
		t.Fatal(err)
	}
	if cache.HostPath != dir || cache.ContainerPath != DefaultCacheContainerPath {
		t.Fatalf("cache = %#v", cache)
	}
	if got := cache.MountArgs(); len(got) != 2 || got[0] != "--mount" {
		t.Fatalf("mount args = %#v", got)
	}
}

func TestResolveCacheDisabled(t *testing.T) {
	t.Setenv("CNB_CACHE_DIR", "")
	t.Setenv("BKS_CACHE_DIR", "")
	cache, err := resolveCache()
	if err != nil {
		t.Fatal(err)
	}
	if cache.Enabled() {
		t.Fatalf("cache should be disabled: %#v", cache)
	}
}

func TestResolveDriver(t *testing.T) {
	t.Setenv("CNB_RUNTIME_DRIVER", "")
	t.Setenv("BKS_RUNTIME_DRIVER", "")
	driver, err := resolveDriver()
	if err != nil || driver != DriverHost {
		t.Fatalf("driver=%s err=%v", driver, err)
	}
	t.Setenv("CNB_RUNTIME_DRIVER", "managed")
	driver, err = resolveDriver()
	if err != nil || driver != DriverManaged {
		t.Fatalf("driver=%s err=%v", driver, err)
	}
	t.Setenv("CNB_RUNTIME_DRIVER", "bad")
	if _, err := resolveDriver(); err == nil {
		t.Fatal("expected invalid driver error")
	}
}
