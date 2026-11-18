package materials

import (
	"os"
	"path/filepath"
	"testing"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

func TestUnpackIDChangesWithRules(t *testing.T) {
	a := &lock.Asset{Unpack: lock.Unpack{Kind: "tgz", Strip: 1, Dir: "x/1", Expect: []string{"bin"}}}
	base := unpackID(a)

	b := *a
	b.Unpack.Strip = 2
	if unpackID(&b) == base {
		t.Fatal("strip 变化应导致指纹变化")
	}
	c := *a
	c.Unpack.Expect = []string{"other"}
	if unpackID(&c) == base {
		t.Fatal("expect 变化应导致指纹变化")
	}
	if unpackID(a) != base {
		t.Fatal("相同配置指纹应稳定")
	}
}

func TestCleanTargetRemovesPreviousExtraction(t *testing.T) {
	bin := t.TempDir()
	dir := filepath.Join(bin, "x", "1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &lock.Asset{Unpack: lock.Unpack{Kind: "tgz", Dir: "x/1"}}
	if err := cleanTarget(a, bin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("旧解包目录应被删除")
	}
}
