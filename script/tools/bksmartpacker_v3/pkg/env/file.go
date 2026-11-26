package env

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadFile 解析 dotenv 文件：KEY=VALUE，支持空行与 # 注释。
func LoadFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: missing '='", path, lineNo)
		}
		key = strings.TrimSpace(key)
		if !ValidKey(key) {
			return nil, fmt.Errorf("%s:%d: invalid key %q", path, lineNo, key)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		out[key] = value
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
