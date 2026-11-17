package executors

import (
	"reflect"
	"testing"
)

func TestSaveArgsPodman(t *testing.T) {
	got := saveArgs("/usr/bin/podman", "/tmp/web.tgz", "bk/web:1")
	want := []string{"image", "save", "--format", "oci-archive", "-o", "/tmp/web.tgz", "bk/web:1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("podman 参数错误: %v", got)
	}
}

func TestSaveArgsDocker(t *testing.T) {
	got := saveArgs("/usr/bin/docker", "/tmp/web.tgz", "bk/web:1")
	want := []string{"image", "save", "-o", "/tmp/web.tgz", "bk/web:1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("docker 参数错误: %v", got)
	}
}
