package builder

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bksmartpacker/pkg/config"
	"bksmartpacker/pkg/types"
)

var app_desc = `specVersion: 3
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

var app_desc_with_build = `specVersion: 3
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
			Expect(config.Init("")).To(BeNil())
			tmpAppPath, _ = os.MkdirTemp("", "test-smart-app")
		})
		AfterEach(func() {
			Expect(os.RemoveAll(tmpAppPath)).To(BeNil())
		})

		When("no build config", func() {
			var appDesc types.ApplicationDescription
			BeforeEach(func() {
				appDescPath := filepath.Join(tmpAppPath, "app_desc.yaml")
				Expect(os.WriteFile(appDescPath, []byte(app_desc), 0o644)).To(BeNil())

				appDesc, _, _ = loadAppDesc(tmpAppPath)
			})

			It("Aptfile exists", func() {
				appDescPath := filepath.Join(tmpAppPath, "Aptfile")
				_, err := os.Create(appDescPath)
				Expect(err).To(BeNil())

				Expect(
					generateModuleBuildpacks(tmpAppPath, &appDesc.Modules[0]),
				).To(Equal([]Buildpack{
					{Name: "bk-buildpack-python", Version: "v213", Type: "oci-embedded"},
					{Name: "bk-buildpack-apt", Version: "v2", Type: "oci-embedded"},
				}))
			})

			It("Aptfile does not exists", func() {
				Expect(
					generateModuleBuildpacks(tmpAppPath, &appDesc.Modules[0]),
				).To(Equal([]Buildpack{{Name: "bk-buildpack-python", Version: "v213", Type: "oci-embedded"}}))
			})
		})

		When("set build config", func() {
			var appDesc types.ApplicationDescription
			BeforeEach(func() {
				appDescPath := filepath.Join(tmpAppPath, "app_desc.yaml")
				Expect(os.WriteFile(appDescPath, []byte(app_desc_with_build), 0o644)).To(BeNil())

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
