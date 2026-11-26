package env

import (
	"maps"
	"strings"

	"github.com/samber/lo"

	"bksmartpacker/pkg/types"
)

// Merge 合并 app_desc 环境变量与命令行注入项，后者覆盖前者。
func Merge(declared []types.AppEnvVar, extra []string) map[string]string {
	out := lo.Associate(declared, func(item types.AppEnvVar) (string, string) {
		return item.Name, item.Value
	})
	for _, kv := range extra {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

// MergeModule 合并 app_desc、env-file、CLI 三层，后者覆盖前者。
func MergeModule(declared []types.AppEnvVar, envFiles []string, extra []string) (map[string]string, error) {
	out := Merge(declared, nil)
	for _, path := range envFiles {
		values, err := LoadFile(path)
		if err != nil {
			return nil, err
		}
		maps.Copy(out, values)
	}
	for _, kv := range extra {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k != "" {
			out[k] = v
		}
	}
	return out, nil
}
