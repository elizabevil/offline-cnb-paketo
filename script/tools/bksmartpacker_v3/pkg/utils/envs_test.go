package utils

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bksmartpacker/pkg/types"
)

var _ = Describe("TestEnv", func() {

	var workspace string
	BeforeEach(func() {
		var err error
		workspace, err = os.MkdirTemp("", "")
		Expect(err).To(BeNil())
	})

	DescribeTable("Test GenerateEnvs", func(appDesc types.ApplicationDescription, expected map[string]string) {
		err := GenerateEnvs(workspace, appDesc)
		Expect(err).To(BeNil())

		for filename, env := range expected {
			data, err := os.ReadFile(filepath.Join(workspace, "Envs", filename))
			Expect(err).To(BeNil())
			Expect(string(data)).To(Equal(env))
		}

	}, Entry("test env var with space", types.ApplicationDescription{
		Modules: []types.ModuleDescription{
			{
				Name: "default",
				Spec: types.BkAppSpec{
					Configuration: types.AppConfig{
						Env: []types.AppEnvVar{
							{
								Name:  "CXX",
								Value: "g++ -I/app/.apt/usr/include/x86_64-linux-gnu/c++/7 -I/app/.apt/usr/include/c++/7 -L/app/.apt/usr/lib/gcc/x86_64-linux-gnu/7",
							},
						},
					},
				},
			},
		},
	}, map[string]string{
		"default.env": "CXX=\"g++ -I/app/.apt/usr/include/x86_64-linux-gnu/c++/7 -I/app/.apt/usr/include/c++/7 -L/app/.apt/usr/lib/gcc/x86_64-linux-gnu/7\"\n",
	}),
		Entry("test env var with dobule quote", types.ApplicationDescription{
			Modules: []types.ModuleDescription{
				{
					Name: "default",
					Spec: types.BkAppSpec{
						Configuration: types.AppConfig{
							Env: []types.AppEnvVar{
								{
									Name:  "CXX",
									Value: "a \"b\" c",
								},
							},
						},
					},
				},
			},
		}, map[string]string{
			"default.env": "CXX=\"a \\\"b\\\" c\"\n",
		}),
	)
})
