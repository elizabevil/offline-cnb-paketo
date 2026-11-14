package lock

import (
	"fmt"
	"os"
	"strings"
)

// SetLocalSHA 回填本地物料的 sha256（[asset.local] 段内）。
func SetLocalSHA(path, ref, actual string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(data), "\n")

	id, version, hasVer := strings.Cut(ref, "@")
	inAsset, match, inLocal := false, false, false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		switch {
		case t == "[[asset]]":
			inAsset, match, inLocal = true, false, false
			continue
		case strings.HasPrefix(t, "["):
			inLocal = strings.HasPrefix(t, "[asset.local]")
			continue
		}
		if !inAsset {
			continue
		}
		if strings.HasPrefix(t, "id = ") && strings.Contains(t, `"`+id+`"`) {
			match = !hasVer
		}
		if strings.HasPrefix(t, "version = ") && strings.Contains(t, `"`+version+`"`) {
			match = true
		}
		if match && inLocal && strings.HasPrefix(t, "sha256 = ") {
			cur := strings.Trim(strings.TrimPrefix(t, "sha256 = "), `"`)
			// 本地物料是构建产物：每次发布都以最新产物为准（直接覆盖）
			if cur == actual {
				return false, nil
			}
			indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
			lines[i] = indent + `sha256 = "` + actual + `"`
			return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}
	return false, fmt.Errorf("未找到 %s 的 [asset.local] 段", ref)
}
