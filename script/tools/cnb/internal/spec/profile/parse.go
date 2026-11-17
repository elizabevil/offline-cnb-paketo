package profile

import (
	"github.com/pelletier/go-toml/v2"

	"learn-cnb-paketo/tools/cnb/internal/spec/warn"
)

// knownKeys 是当前支持的顶层键。
var knownKeys = map[string]bool{
	"apiVersion": true, "kind": true, "metadata": true, "spec": true,
}

// Parse 解析流水线配置；未知顶层键以 Warn 返回。
func Parse(data []byte) (*File, []warn.Warning, error) {
	var f File
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, nil, err
	}
	var raw map[string]any
	var out []warn.Warning
	if err := toml.Unmarshal(data, &raw); err == nil {
		for k := range raw {
			if !knownKeys[k] {
				out = append(out, warn.Warning{Path: k, Message: "unknown top-level key in profile"})
			}
		}
	}
	return &f, out, nil
}
