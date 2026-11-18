package imagefmt

import (
	"encoding/json"
	"os"
)

// writeJSON 写出 JSON 文件。
func writeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
