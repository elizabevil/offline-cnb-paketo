package lock

import (
	"bytes"
	"io"
	"testing"
)

func asset(id, ver string, def bool) Asset {
	return Asset{
		ID: id, Version: ver, Default: def,
		Unpack:   Unpack{Kind: "tgz", Dir: id + "/{version}", Expect: []string{"x"}},
		Platform: map[string]Source{"linux-amd64": {URL: "https://example.com/x.tgz"}},
	}
}

func TestValidate(t *testing.T) {
	ok := &File{
		Schema: 1,
		Meta:   Meta{Platforms: []string{"linux-amd64", "linux-arm64"}},
		Assets: []Asset{asset("lifecycle", "0.21.13", true)},
	}
	if n := ok.Validate(); n != 0 {
		t.Fatalf("合法 lock 被判为非法，错误数=%d", n)
	}

	bad := &File{
		Schema: 2,
		Meta:   Meta{Platforms: []string{"linux-amd64"}},
		Assets: []Asset{
			{ID: "dup", Version: "", Unpack: Unpack{Kind: "rar"}},
			{ID: "dup", Version: "1", Unpack: Unpack{Kind: "tgz", Expect: []string{"x"}},
				Platform: map[string]Source{"linux-arm64": {URL: "not-a-url"}}},
		},
	}
	if n := bad.Validate(); n == 0 {
		t.Fatal("非法 lock 未被识别")
	}
}

func TestListColumns(t *testing.T) {
	f := &File{Assets: []Asset{asset("fnm", "1.39.0", true)}}
	var buf bytes.Buffer
	WriteList(&buf, f)
	want := "fnm\t1.39.0\ttgz\tlinux-amd64\n"
	if buf.String() != want {
		t.Fatalf("list 输出不符合契约\n got: %q\nwant: %q", buf.String(), want)
	}
}

func TestGetUnknownAsset(t *testing.T) {
	f := &File{Assets: []Asset{asset("a", "1", true)}}
	if err := WriteShell(io.Discard, f, "nope", "linux-amd64"); err == nil {
		t.Fatal("未知物料应返回错误")
	}
}

func TestFindByRefAndDefault(t *testing.T) {
	f := &File{Assets: []Asset{
		asset("lifecycle", "0.21.13", true),
		asset("lifecycle", "0.17.6", false),
	}}
	if a, err := Find(f, "lifecycle"); err != nil || a.Version != "0.21.13" {
		t.Fatalf("默认版本选中错误: %v", err)
	}
	if a, err := Find(f, "lifecycle@0.17.6"); err != nil || a.Version != "0.17.6" {
		t.Fatalf("显式版本选中错误: %v", err)
	}
}

func TestDirExpandsVersion(t *testing.T) {
	a := asset("lifecycle", "0.21.13", true)
	if got := a.Dir(); got != "lifecycle/0.21.13" {
		t.Fatalf("dir 展开错误: %s", got)
	}
	if got := a.ExpectedPaths(); len(got) != 1 || got[0] != "lifecycle/0.21.13/x" {
		t.Fatalf("expect 展开错误: %v", got)
	}
}
