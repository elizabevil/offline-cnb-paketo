package manifest

import (
	"github.com/pelletier/go-toml/v2"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// knownKeys 是当前支持的顶层键；未知键会产出告警。
var knownKeys = map[string]bool{
	"schema": true, "meta": true, "storage": true, "asset": true,
}

// Parse 解析 assets.lock.toml；未知顶层键以 Warn 返回。
func Parse(data []byte) (*File, []errs.Warning, error) {
	var f File
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, nil, err
	}
	var raw map[string]any
	var out []errs.Warning
	if err := toml.Unmarshal(data, &raw); err == nil {
		for k := range raw {
			if !knownKeys[k] {
				out = append(out, errs.Warning{Path: k, Message: "unknown top-level key in assets.lock.toml"})
			}
		}
	}
	return &f, out, nil
}

// Encode 序列化为 TOML（仅用于生成模板）。
func (f *File) Encode() ([]byte, error) { return toml.Marshal(f) }
