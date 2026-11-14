// Package repack 的兼容性测试。
package repack

// 内部测试：解析、有序 map 与紧凑 JSON 的行为。

import "testing"

func TestParseRepoTag(t *testing.T) {
	cases := []struct {
		in       string
		wantRepo string
		wantTag  string
	}{
		{"docker.io/bk_cmdb_saas/web:3.14.7-zts", "bk_cmdb_saas/web", "3.14.7-zts"},
		{"bk_cmdb_saas/web:3.14.7-zts", "bk_cmdb_saas/web", "3.14.7-zts"},
		{"bk_cmdb_saas/web", "bk_cmdb_saas/web", "latest"},
		{"localhost:5000/team/app:1.0", "localhost:5000/team/app", "1.0"},
		{"docker.io/library/nginx:1.25", "library/nginx", "1.25"},
	}
	for _, c := range cases {
		repo, tag := parseRepoTag(c.in)
		if repo != c.wantRepo || tag != c.wantTag {
			t.Errorf("parseRepoTag(%q) = (%q, %q)，期望 (%q, %q)", c.in, repo, tag, c.wantRepo, c.wantTag)
		}
	}
}

func TestLayerSourcesKeepsOrder(t *testing.T) {
	src := layerSources{items: []layerSource{
		{digest: "sha256:bbb", desc: blobDescriptor{MediaType: "m", Digest: "sha256:bbb", Size: 2}},
		{digest: "sha256:aaa", desc: blobDescriptor{MediaType: "m", Digest: "sha256:aaa", Size: 1}},
	}}
	data, err := marshalCompact(src)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"sha256:bbb":{"mediaType":"m","digest":"sha256:bbb","size":2},"sha256:aaa":{"mediaType":"m","digest":"sha256:aaa","size":1}}`
	if string(data) != want {
		t.Fatalf("顺序或格式不符\n got: %s\nwant: %s", data, want)
	}
}

func TestMarshalCompactNoHTMLDigits(t *testing.T) {
	// 确认编码器不转义 HTML 字符（与 Python ensure_ascii=False 对齐）
	got, err := marshalCompact(map[string]string{"k": "a<b&c>"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"k":"a<b&c>"}` {
		t.Fatalf("HTML 转义不符合预期: %s", got)
	}
}
