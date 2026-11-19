package checks

import (
	"os"
	"path/filepath"
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
