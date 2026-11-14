package buildmeta

import (
	"os"

	"learn-cnb-paketo/tools/cnb/internal/profile"
)

// StackID 从构建该镜像的 Dockerfile 里读 stack id 标签。
func StackID(plan *profile.Plan) string {
	for _, st := range plan.Stages {
		if st.Kind != profile.StageImage {
			continue
		}
		data, err := os.ReadFile(st.Dockerfile)
		if err != nil {
			continue
		}
		if m := labelRe.FindStringSubmatch(string(data)); m != nil {
			return m[1]
		}
	}
	return ""
}
