// Package render 提供日志器与结构化输出（yaml / json）。
package render

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// 输出格式名（命令行与内部共用）。
const (
	// FormatJSON 是 JSON 输出。
	FormatJSON = "json"
	// FormatYAML 是 YAML 输出。
	FormatYAML = "yaml"
)

// FormatStruct 把任意结构体按 yaml / json 输出。
func FormatStruct(v any, format string) (string, error) {
	switch format {
	case FormatJSON:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	case FormatYAML, "":
		b, err := yaml.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "", fmt.Errorf("unsupported format %q (yaml|json)", format)
	}
}
