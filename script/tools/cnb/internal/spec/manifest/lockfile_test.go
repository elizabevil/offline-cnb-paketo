package manifest

import "testing"

func TestMarshalParseLockRoundTrip(t *testing.T) {
	in := &LockFile{
		Schema:   1,
		Manifest: LockManifest{SHA256: "abc"},
		Locked: []LockedAsset{{
			ID: "x", Version: "1",
			Platform: map[string]LockedSource{"linux-amd64": {URL: "https://e/x", Size: 3, SHA256: "def"}},
		}},
	}
	data, err := MarshalLock(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ParseLock(data)
	if err != nil {
		t.Fatal(err)
	}
	if out.Manifest.SHA256 != "abc" || len(out.Locked) != 1 || out.Locked[0].Platform["linux-amd64"].SHA256 != "def" {
		t.Fatalf("往返不一致: %+v", out)
	}
}

func TestHashStable(t *testing.T) {
	f := &File{Schema: 1, Meta: Meta{Name: "n"}, Assets: []Asset{{ID: "x", Version: "1"}}}
	a, err := Hash(f)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Hash(f)
	if a == "" || a != b {
		t.Fatalf("hash 不稳定: %q %q", a, b)
	}
}
