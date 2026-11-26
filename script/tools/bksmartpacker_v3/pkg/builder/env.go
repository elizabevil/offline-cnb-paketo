package builder

import (
	"maps"
	"strings"

	"github.com/samber/lo"

	"bksmartpacker/pkg/types"
)

// mergeEnvs 合并 app_desc 环境变量与命令行注入项，后者覆盖前者。
func mergeEnvs(declared []types.AppEnvVar, extra []string) map[string]string {
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

// mergeModuleEnvs 合并 app_desc、env-file、CLI 三层，后者覆盖前者。
func mergeModuleEnvs(declared []types.AppEnvVar, envFiles []string, extra []string) (map[string]string, error) {
	out := mergeEnvs(declared, nil)
	for _, path := range envFiles {
		values, err := LoadEnvFile(path)
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
