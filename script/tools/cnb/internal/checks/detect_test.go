package checks

import "testing"

func TestIsShellScript(t *testing.T) {
	cases := []struct {
		argv []string
		want bool
	}{
		{[]string{"bash", "build.sh"}, true},
		{[]string{"/bin/sh", "run.sh"}, true},
		{[]string{"bash", "-c", "echo hi"}, false},
		{[]string{"python", "app.py"}, false},
		{[]string{"bash"}, false},
	}
	for _, c := range cases {
		if got := IsShellScript(c.argv); got != c.want {
			t.Errorf("IsShellScript(%v) = %v, want %v", c.argv, got, c.want)
		}
	}
}
