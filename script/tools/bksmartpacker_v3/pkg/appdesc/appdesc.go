package appdesc

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"gopkg.in/yaml.v3"

	"bksmartpacker/pkg/types"
	"bksmartpacker/pkg/utils"
)

const (
	appDescFileName    = "app_desc.yaml"
	appDescAltFileName = "app_desc.yml"
)

// Load 读取并解析 app_desc.yaml。
func Load(src string) (types.ApplicationDescription, string, error) {
	if err := ensureYAML(src); err != nil {
		return types.ApplicationDescription{}, "", errors.Wrap(err, appDescFileName+" not found")
	}

	data, err := os.ReadFile(filepath.Join(src, appDescFileName))
	if err != nil {
		return types.ApplicationDescription{}, "", errors.Wrap(err, "failed to read "+appDescFileName)
	}
	appDesc := types.ApplicationDescription{}
	if err := yaml.Unmarshal(data, &appDesc); err != nil {
		return types.ApplicationDescription{}, "", errors.Wrap(err, "failed to parse "+appDescFileName)
	}
	return appDesc, string(data), nil
}

// ensureAppDescYaml 保证 app_desc.yaml 存在；兼容 app_desc.yml。
func ensureYAML(src string) error {
	path := filepath.Join(src, appDescFileName)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	alt := filepath.Join(src, appDescAltFileName)
	if _, err := os.Stat(alt); err != nil {
		return err
	}
	if err := utils.Copy(alt, path); err != nil {
		return err
	}
	return os.Remove(alt)
}

// Filter 按 --module 过滤模块；部分包默认禁止生成。
func Filter(appDesc *types.ApplicationDescription, modules []string, allowPartial bool) error {
	if len(modules) == 0 {
		return nil
	}
	if !allowPartial {
		return fmt.Errorf("--module generates a partial package which cannot be deployed; use --allow-partial for debug builds")
	}
	want := lo.SliceToMap(modules, func(name string) (string, bool) { return name, true })
	filtered := make([]types.ModuleDescription, 0, len(modules))
	for _, m := range appDesc.Modules {
		if want[m.Name] {
			filtered = append(filtered, m)
			delete(want, m.Name)
		}
	}
	if len(want) > 0 {
		return fmt.Errorf("module not found: %v", lo.Keys(want))
	}
	appDesc.Modules = filtered
	return nil
}
