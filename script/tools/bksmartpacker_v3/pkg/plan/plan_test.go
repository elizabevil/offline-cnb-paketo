package plan

import (
	"sort"
	"testing"
)

func TestModuleBuildStepBuildEnvs(t *testing.T) {
	cases := []struct {
		name string
		envs map[string]string
		want []string
	}{
		{
			name: "spaces",
			envs: map[string]string{"CXX": "g++ -I/app/.apt/usr/include/x86_64-linux-gnu/c++/7 -I/app/.apt/usr/include/c++/7 -L/app/.apt/usr/lib/gcc/x86_64-linux-gnu/7"},
			want: []string{"CXX=g++ -I/app/.apt/usr/include/x86_64-linux-gnu/c++/7 -I/app/.apt/usr/include/c++/7 -L/app/.apt/usr/lib/gcc/x86_64-linux-gnu/7"},
		},
		{
			name: "quotes",
			envs: map[string]string{"CXX": `a "b" c`},
			want: []string{`CXX=a "b" c`},
		},
		{
			name: "multiple",
			envs: map[string]string{
				"GO_INSTALL_PACKAGE_SPEC": "./src/web_server",
				"CGO_CFLAGS":              "-I/app/Tongsuo-8.3.2/include -Wno-deprecated-declarations",
				"CGO_LDFLAGS":             "-L/opt/tongsuo/lib -lssl -lcrypto",
			},
			want: []string{
				"GO_INSTALL_PACKAGE_SPEC=./src/web_server",
				"CGO_CFLAGS=-I/app/Tongsuo-8.3.2/include -Wno-deprecated-declarations",
				"CGO_LDFLAGS=-L/opt/tongsuo/lib -lssl -lcrypto",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ModuleBuildStep{Envs: tc.envs}.BuildEnvs()
			sort.Strings(got)
			sort.Strings(tc.want)
			if len(got) != len(tc.want) {
				t.Fatalf("envs = %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("envs = %#v, want %#v", got, tc.want)
				}
			}
		})
	}
}
