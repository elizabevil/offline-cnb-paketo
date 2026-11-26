package builder

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bksmartpacker/pkg/types"
)

var appDescYAML = `specVersion: 3
appVersion: 0.0.1
app:
  bkAppCode: "test-app"
  bkAppName: "测试应用"
  market:
    introduction: no body care
modules:
  - name: main
    language: "Python"
    isDefault: true
    spec:
      processes:
        - name: web
          procCommand: python -m uvicorn app:app --host 0.0.0.0 --port 5000
      hooks:
        preRelease:
          procCommand: python -c 'import time;[print(i) or time.sleep(1) for i in range(10)]'
`

var appDescWithBuildYAML = `specVersion: 3
appVersion: 0.0.1
app:
  bkAppCode: "test-app"
  bkAppName: "测试应用"
  market:
    introduction: no body care
modules:
  - name: main
    language: "Python"
    isDefault: true
    spec:
      processes:
        - name: web
          procCommand: python -m uvicorn app:app --host 0.0.0.0 --port 5000
      hooks:
        preRelease:
          procCommand: python -c 'import time;[print(i) or time.sleep(1) for i in range(10)]'
      build:
        buildpacks:
          - name: bk-buildpack-python
            version: "v-test"
`

var _ = Describe("Test prepare", func() {
	Describe("test generateModuleBuildpacks", func() {
		var tmpAppPath string

		BeforeEach(func() {
			tmpAppPath, _ = os.MkdirTemp("", "test-smart-app")
		})
		AfterEach(func() {
			Expect(os.RemoveAll(tmpAppPath)).To(BeNil())
		})

		When("no build config", func() {
			var appDesc types.ApplicationDescription
			BeforeEach(func() {
				appDescPath := filepath.Join(tmpAppPath, "app_desc.yaml")
				Expect(os.WriteFile(appDescPath, []byte(appDescYAML), 0o644)).To(BeNil())

				appDesc, _, _ = loadAppDesc(tmpAppPath)
			})

			It("Aptfile exists", func() {
				appDescPath := filepath.Join(tmpAppPath, "Aptfile")
				_, err := os.Create(appDescPath)
				Expect(err).To(BeNil())

				Expect(
					generateModuleBuildpacks(tmpAppPath, &appDesc.Modules[0]),
				).To(Equal([]Buildpack{{Name: "paketo-buildpacks/python"}}))
			})

			It("Aptfile does not exists", func() {
				Expect(
					generateModuleBuildpacks(tmpAppPath, &appDesc.Modules[0]),
				).To(Equal([]Buildpack{{Name: "paketo-buildpacks/python"}}))
			})
		})

		When("set build config", func() {
			var appDesc types.ApplicationDescription
			BeforeEach(func() {
				appDescPath := filepath.Join(tmpAppPath, "app_desc.yaml")
				Expect(os.WriteFile(appDescPath, []byte(appDescWithBuildYAML), 0o644)).To(BeNil())

				appDesc, _, _ = loadAppDesc(tmpAppPath)
			})

			It("Aptfile exists", func() {
				appDescPath := filepath.Join(tmpAppPath, "Aptfile")
				_, err := os.Create(appDescPath)
				Expect(err).To(BeNil())

				Expect(
					generateModuleBuildpacks(tmpAppPath, &appDesc.Modules[0]),
				).To(Equal([]Buildpack{
					{Name: "bk-buildpack-python", Version: "v-test"},
				}))
			})

			It("Aptfile does not exists", func() {
				Expect(
					generateModuleBuildpacks(tmpAppPath, &appDesc.Modules[0]),
				).To(Equal([]Buildpack{{Name: "bk-buildpack-python", Version: "v-test"}}))
			})
		})
	})
})
