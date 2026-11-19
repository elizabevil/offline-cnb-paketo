package checks

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCopySourcesSkipsStagesAndFlags(t *testing.T) {
	dir := t.TempDir()
	df := filepath.Join(dir, "Dockerfile")
	content := `FROM debian
COPY --from=builder /out/app /usr/local/bin/app
COPY --chown=0:0 docker/a.txt /etc/a.txt
COPY docker/b/ /etc/b/
ADD --chmod=755 script.sh /usr/local/bin/script.sh
RUN echo hi
`
	if err := os.WriteFile(df, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := copySources(df)
	want := []copySource{
		{from: "builder", path: "/out/app"},
		{from: "", path: "docker/a.txt"},
		{from: "", path: "docker/b/"},
		{from: "", path: "script.sh"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d sources, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("source %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCheckDockerfilesReportsMissing(t *testing.T) {
	repo := t.TempDir()
	ctx := filepath.Join(repo, "project")
	if err := os.MkdirAll(filepath.Join(ctx, "docker", "present"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctx, "docker", "present", "a.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	df := filepath.Join(ctx, "Dockerfile")
	content := "FROM debian\nCOPY docker/present/a.txt /a\nCOPY docker/missing/b.txt /b\nCOPY ${UNKNOWN}/x /x\n"
	if err := os.WriteFile(df, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := CheckDockerfiles([]DockerfileTarget{{Name: "builder", Dockerfile: df, Context: ctx}}, "", "")
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].Source != "docker/missing/b.txt" {
		t.Errorf("unexpected source: %s", got[0].Source)
	}
}

func TestCheckDockerfilesExpandsVariables(t *testing.T) {
	dir := t.TempDir()
	ctx := filepath.Join(dir, "project")
	if err := os.MkdirAll(filepath.Join(ctx, "binaries", "v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	df := filepath.Join(ctx, "Dockerfile")
	content := "ARG ARCH=amd64\nARG VERSION\nENV DIR=binaries\nCOPY ${DIR}/${VERSION}/ /out/\nCOPY ${ARCH}/missing/ /out2/\n"
	if err := os.WriteFile(df, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := CheckDockerfiles([]DockerfileTarget{{
		Name: "builder", Dockerfile: df, Context: ctx,
		Args: map[string]string{"VERSION": "v1"},
	}}, "", "")
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].Source != "amd64/missing/" {
		t.Errorf("unexpected source: %s", got[0].Source)
	}
}

func TestExpandVarsLeavesUnknown(t *testing.T) {
	got := expandVars("/${KNOWN}/${UNKNOWN}/x", map[string]string{"KNOWN": "k"})
	if want := "/k/${UNKNOWN}/x"; got != want {
		t.Errorf("expandVars = %q, want %q", got, want)
	}
}

func TestUnknownFromStageReported(t *testing.T) {
	dir := t.TempDir()
	df := filepath.Join(dir, "Dockerfile")
	content := "FROM debian AS builder\nFROM debian AS final\nCOPY --from=builer /out/app /app\nCOPY --from=builder /out/lib /lib\nCOPY --from=alpine:3.19 /bin/sh /sh\n"
	if err := os.WriteFile(df, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := CheckDockerfiles([]DockerfileTarget{{Name: "img", Dockerfile: df, Context: dir}}, "", "")
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Reason, "builer") {
		t.Errorf("unexpected finding: %+v", got[0])
	}
}

func TestStageNamesParsed(t *testing.T) {
	dir := t.TempDir()
	df := filepath.Join(dir, "Dockerfile")
	content := "FROM --platform=linux/amd64 debian AS Base\nFROM Base AS final\nFROM scratch\n"
	if err := os.WriteFile(df, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := stageNames(df)
	if want := []string{"Base", "final"}; !slices.Equal(got, want) {
		t.Errorf("stageNames = %v, want %v", got, want)
	}
}
