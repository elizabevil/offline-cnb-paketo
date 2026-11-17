// Package lock 的读取与查找。
package lock

import (
	"os"

	"learn-cnb-paketo/tools/cnb/internal/errs"
	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
)

// Load 读取并解析物料清单。
func Load(path string) (*File, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, &errs.NotFound{Kind: "lock file", Ref: path}
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil, readErr
	}
	f, _, parseErr := manifest.Parse(data)
	if parseErr != nil {
		return nil, errs.Wrap("parse toml", parseErr)
	}
	return f, nil
}

// List 输出 TSV：id version kind platforms。
