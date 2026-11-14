package shell

import "testing"

func TestQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "''"},
		{"abc", "'abc'"},
		{"a b", "'a b'"},
		{"it's", `'it'\''s'`},
		{`BACK\SLASH`, `'BACK\SLASH'`},
	}
	for _, c := range cases {
		if got := Quote(c.in); got != c.want {
			t.Errorf("Quote(%q) = %s，期望 %s", c.in, got, c.want)
		}
	}
}
