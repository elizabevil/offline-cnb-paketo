package manifest

import "testing"

func baseAsset(size int64) Asset {
	return Asset{
		ID: "x", Version: "1",
		Unpack:   Unpack{Kind: "tgz", Dir: "x/{version}", Expect: []string{"bin"}},
		Platform: map[string]Source{"linux-amd64": {URL: "https://e/x.tgz", Size: size}},
	}
}

func TestSizeUnknownAllowed(t *testing.T) {
	f := &File{Schema: 1, Meta: Meta{Platforms: []string{"linux-amd64"}}, Assets: []Asset{baseAsset(-1)}}
	if n := f.Validate(); n != 0 {
		t.Fatalf("-1 应允许，错误数=%d", n)
	}
}

func TestSizeBelowMinusOneRejected(t *testing.T) {
	f := &File{Schema: 1, Meta: Meta{Platforms: []string{"linux-amd64"}}, Assets: []Asset{baseAsset(-2)}}
	if n := f.Validate(); n == 0 {
		t.Fatal("小于 -1 应被拒绝")
	}
}
