package lifecycle

import "testing"

func TestNormalizeRef(t *testing.T) {
	cases := []struct {
		in, repo, tag, digest string
		bad                   bool
	}{
		{"run:latest", "index.docker.io/library/run", "latest", "", false},
		{"run", "index.docker.io/library/run", "", "", false},
		{"run@sha256:abc", "index.docker.io/library/run", "", "sha256:abc", false},
		{"localhost:5000/team/run:1.0", "localhost:5000/team/run", "1.0", "", false},
		{"library/run:1", "index.docker.io/library/run", "1", "", false},
		{"", "", "", "", true},
	}
	for _, c := range cases {
		repo, tag, digest, err := NormalizeRef(c.in)
		if c.bad {
			if err == nil {
				t.Fatalf("%q 应报错", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if repo != c.repo || tag != c.tag || digest != c.digest {
			t.Fatalf("%q => (%q,%q,%q), 期望 (%q,%q,%q)", c.in, repo, tag, digest, c.repo, c.tag, c.digest)
		}
	}
}
