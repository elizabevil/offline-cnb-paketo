package lifecycle

import (
	"os"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	speclifecycle "learn-cnb-paketo/tools/cnb/internal/spec/lifecycle"
)

// Aliases 把外部平台的 buildpack 名映射到本项目 order 里的 id。
// 命中判定会同时尝试原名与别名，因此不同平台的 order 写法都能匹配。
var Aliases = map[string]string{
	"bk-buildpack-go":     "paketo-buildpacks/go",
	"bk-buildpack-nodejs": "paketo-buildpacks/nodejs",
	"bk-buildpack-python": "paketo-buildpacks/python",
}

// FilterOrder 读取 src，只保留被 required 命中的 order 块；为空则原样返回。
func FilterOrder(src, required string, aliases map[string]string) ([]byte, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return nil, err
	}
	doc, _, err := speclifecycle.ParseOrder(data)
	if err != nil {
		return nil, errs.Wrap("parse "+src, err)
	}
	kept := doc.Filter(required, aliases)
	if required == "" {
		return data, nil
	}
	return kept.Encode()
}
