package builder

import (
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test ModuleBuildStep", func() {
	DescribeTable("Test BuildEnvs", func(step ModuleBuildStep, expected []string) {
		envs := step.BuildEnvs()
		sort.Strings(envs)
		sort.Strings(expected)
		Expect(envs).To(Equal(expected))
	}, Entry("test env var with space",
		ModuleBuildStep{
			Envs: map[string]string{
				"CXX": "g++ -I/app/.apt/usr/include/x86_64-linux-gnu/c++/7 -I/app/.apt/usr/include/c++/7 -L/app/.apt/usr/lib/gcc/x86_64-linux-gnu/7",
			},
		},
		[]string{
			"CXX=g++ -I/app/.apt/usr/include/x86_64-linux-gnu/c++/7 -I/app/.apt/usr/include/c++/7 -L/app/.apt/usr/lib/gcc/x86_64-linux-gnu/7",
		}),
		Entry("test env var with double quote",
			ModuleBuildStep{
				Envs: map[string]string{
					"CXX": "a \"b\" c",
				},
			}, []string{"CXX=a \"b\" c"}),
		Entry("test multiple envs",
			ModuleBuildStep{
				Envs: map[string]string{
					"GO_INSTALL_PACKAGE_SPEC": "./src/web_server",
					"CGO_CFLAGS":              "-I/app/Tongsuo-8.3.2/include -Wno-deprecated-declarations",
					"CGO_LDFLAGS":             "-L/opt/tongsuo/lib -lssl -lcrypto",
				},
			}, []string{
				"GO_INSTALL_PACKAGE_SPEC=./src/web_server",
				"CGO_CFLAGS=-I/app/Tongsuo-8.3.2/include -Wno-deprecated-declarations",
				"CGO_LDFLAGS=-L/opt/tongsuo/lib -lssl -lcrypto",
			}),
	)
})
