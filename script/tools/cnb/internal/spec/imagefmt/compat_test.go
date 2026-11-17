package imagefmt

import "testing"

func TestSplitRepoTag(t *testing.T) {
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
		repo, tag := splitRepoTag(c.in)
		if repo != c.wantRepo || tag != c.wantTag {
			t.Errorf("splitRepoTag(%q) = (%q, %q)，期望 (%q, %q)", c.in, repo, tag, c.wantRepo, c.wantTag)
		}
	}
}
