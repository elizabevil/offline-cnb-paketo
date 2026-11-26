package plan

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"

	"bksmartpacker/pkg/types"
)

var (
	moduleNameRe  = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	processNameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
)

// GenerateAppProcfile 生成应用级 Procfile，key 为 <module>-<process>。
func GenerateAppProcfile(modules []types.ModuleDescription) (string, error) {
	procfile := make(map[string]string)
	for _, module := range modules {
		if !moduleNameRe.MatchString(module.Name) {
			return "", fmt.Errorf("invalid module name %q: must match %s", module.Name, moduleNameRe.String())
		}
		for _, process := range module.Spec.Processes {
			if process.Name == "" || process.ProcCommand == "" {
				return "", fmt.Errorf("module %s process %q has empty name/command", module.Name, process.Name)
			}
			if !processNameRe.MatchString(process.Name) {
				return "", fmt.Errorf("invalid process name %q in module %s: must match %s", process.Name, module.Name, processNameRe.String())
			}
			procfile[module.Name+"-"+process.Name] = process.ProcCommand
		}
	}
	data, err := yaml.Marshal(procfile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// generateProcfile 生成单模块 Procfile（仅 v1 使用）。
func generateProcfile(moduleDesc types.ModuleDescription) (string, error) {
	procfile := make(map[string]string)
	for _, process := range moduleDesc.Spec.Processes {
		if process.ProcCommand == "" {
			return "", fmt.Errorf("process %s has empty procCommand", process.Name)
		}
		procfile[process.Name] = process.ProcCommand
	}
	data, err := yaml.Marshal(procfile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
