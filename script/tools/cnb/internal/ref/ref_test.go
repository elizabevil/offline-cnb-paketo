package ref

import "testing"

func TestParse(t *testing.T) {
	cases := []struct{ in, id, ver string }{
		{"lifecycle@0.21.29", "lifecycle", "0.21.29"},
		{"cnb@3.0.0", "cnb", "3.0.0"},
		{"locale", "locale", ""},
	}
	for _, c := range cases {
		r, err := Parse(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.in, err)
		}
		if r.ID != c.id || r.Version != c.ver {
			t.Fatalf("%s → %+v", c.in, r)
		}
		if r.String() != c.in {
			t.Fatalf("round-trip 失败: %s → %s", c.in, r.String())
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{"", "@1.0", "id@"} {
		if _, err := Parse(in); err == nil {
			t.Fatalf("%q 应报错", in)
		}
	}
}

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"*", "lifecycle@0.21.29", false}, // * 不跨 separator
		{"**", "lifecycle@0.21.29", true}, // ** 可跨
		{"lifecycle@*", "lifecycle@0.21.29", true},
		{"lifecycle@**", "lifecycle@0.21.29", true},
		{"**/docker", "docker/docker", true},
		{"docker/*", "docker/runc", true},
		{"docker/*", "docker/sub/runc", false},
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.s); got != c.want {
			t.Fatalf("%q 匹配 %q = %v，期望 %v", c.pattern, c.s, got, c.want)
		}
	}
}
