package lock

import (
	"fmt"
	"os"
	"strings"
)

// SetSHA 把某平台的实际哈希写回 lock 文本。
//
// 语义（与 fetch 配合形成"自动固定"）：
//   - lock 中为空  → 写入实际值，返回 changed=true
//   - 已有且相同   → 不动，返回 false
//   - 已有但不同   → 返回错误（下载物被篡改或版本写错）
//
// 采用文本级替换而非整体重编码，保留注释与字段顺序。
func SetSHA(path, ref, platform, actual string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(data), "\n")

	id, version, hasVer := strings.Cut(ref, "@")
	inBlock, match := false, false
	section := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "[[asset]]":
			inBlock, match = true, false
			continue
		case strings.HasPrefix(trimmed, "[asset.platform."):
			section = strings.TrimSuffix(strings.TrimPrefix(trimmed, "[asset.platform."), "]")
			continue
		case strings.HasPrefix(trimmed, "["):
			section = ""
		}
		if !inBlock {
			continue
		}
		if strings.HasPrefix(trimmed, "id = ") && strings.Contains(trimmed, `"`+id+`"`) {
			match = true
			if hasVer {
				match = false // 等 version 行确认
			}
		}
		if strings.HasPrefix(trimmed, "version = ") && match == false && hasVer {
			if strings.Contains(trimmed, `"`+version+`"`) {
				match = true
			}
		}
		if match && section == platform && strings.HasPrefix(trimmed, "sha256 = ") {
			current := strings.Trim(strings.TrimPrefix(trimmed, "sha256 = "), `"`)
			switch {
			case current == actual:
				return false, nil
			case current == "":
				indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
				lines[i] = indent + `sha256 = "` + actual + `"`
				if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
					return false, err
				}
				return true, nil
			default:
				return false, fmt.Errorf("%s[%s]: lock 中 sha256 与下载物不一致\n  lock: %s\n  实际: %s", ref, platform, current, actual)
			}
		}
	}
	return false, fmt.Errorf("未在 lock 中找到 %s 的 [asset.platform.%s] 段", ref, platform)
}
