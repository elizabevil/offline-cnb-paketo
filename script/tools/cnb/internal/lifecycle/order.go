package lifecycle

import (
	"os"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	speclifecycle "learn-cnb-paketo/tools/cnb/internal/spec/lifecycle"
)

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
	if len(kept.Order) == 0 {
		return nil, &errs.Invalid{Field: "REQUIRED_BUILDPACKS", Value: required, Reason: "no matching order entries"}
	}
	return kept.Encode()
}
