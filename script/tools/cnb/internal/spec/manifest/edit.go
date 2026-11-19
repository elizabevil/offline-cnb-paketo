package manifest

import (
	"os"
	"regexp"
	"strings"
)

// EditVersion 只修改指定物料块中的 version 行，保留其余内容与注释。
func EditVersion(path, id, oldVersion, newVersion string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(data), "\n")
	idRe := regexp.MustCompile(`^\s*id\s*=\s*["']` + regexp.QuoteMeta(id) + `["']\s*$`)
	verRe := regexp.MustCompile(`^(\s*version\s*=\s*["'])([^"']*)(["']\s*)(.*)$`)

	inBlock := false
	seenID := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[[asset]]") {
			inBlock, seenID = true, false
			continue
		}
		if !inBlock {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "[") && !strings.HasPrefix(strings.TrimSpace(line), "[[asset]]") {
			inBlock = false
			continue
		}
		if idRe.MatchString(line) {
			seenID = true
			continue
		}
		if !seenID {
			continue
		}
		m := verRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if oldVersion != "" && m[2] != oldVersion {
			continue
		}
		lines[i] = m[1] + newVersion + m[3] + m[4]
		return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
	}
	return false, &NotFoundInManifest{ID: id}
}

// NotFoundInManifest 表示清单中没有对应物料。
type NotFoundInManifest struct{ ID string }

func (e *NotFoundInManifest) Error() string { return e.ID + " not found in assets.toml" }
