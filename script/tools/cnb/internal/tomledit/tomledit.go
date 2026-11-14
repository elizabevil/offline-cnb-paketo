// Package tomledit 在保留注释与排版的前提下修改 TOML 的单个键值。
//
// 只做一件事：给定文件、段落定位条件、键名和新值，就地替换或插入。
// 不解析整个文档语义，因此不会重排、不会丢注释。
package tomledit

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// Section 是段落定位条件。
type Section struct {
	// Header 精确匹配的段头，如 "[asset.local]"。
	Header string
	// WithinAsset 为真时限定在最近一个 "[[asset]]" 之内。
	WithinAsset bool
	// MatchID 非空时要求该 asset 段的 id 等于此值。
	MatchID string
	// MatchVersion 非空时要求 version 等于此值。
	MatchVersion string
}

// Set 修改或插入键值。
func Set(path string, sec Section, key, value string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(data), "\n")
	idx, insertAt := locate(lines, sec)
	if idx < 0 {
		return false, &errs.NotFound{Kind: "toml section", Ref: fmt.Sprintf("%s id=%s version=%s", sec.Header, sec.MatchID, sec.MatchVersion)}
	}

	keyRe := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(key) + `\s*=\s*"(.*)"\s*$`)
	for i := idx + 1; i < len(lines); i++ {
		if isSectionStart(lines[i]) {
			break
		}
		if m := keyRe.FindStringSubmatch(lines[i]); m != nil {
			if m[1] == value {
				return false, nil
			}
			indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " "))]
			lines[i] = fmt.Sprintf(`%s%s = "%s"`, indent, key, value)
			return true, write(path, lines)
		}
		insertAt = i + 1
	}
	lines = append(lines[:insertAt], append([]string{fmt.Sprintf(`  %s = "%s"`, key, value)}, lines[insertAt:]...)...)
	return true, write(path, lines)
}

// locate 返回段落头所在行号与插入位置。
func locate(lines []string, sec Section) (int, int) {
	assetStart, id, version := -1, "", ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[[asset]]" {
			assetStart, id, version = i, "", ""
			continue
		}
		if strings.HasPrefix(trimmed, "id = ") {
			id = unquote(trimmed)
		}
		if strings.HasPrefix(trimmed, "version = ") {
			version = unquote(trimmed)
		}
		if trimmed == sec.Header && assetMatches(sec, id, version) {
			return i, i + 1
		}
	}
	_ = assetStart
	return -1, -1
}

// assetMatches 判断当前 asset 是否满足定位条件。
func assetMatches(sec Section, id, version string) bool {
	if !sec.WithinAsset && sec.MatchID == "" && sec.MatchVersion == "" {
		return true
	}
	if sec.MatchID != "" && id != sec.MatchID {
		return false
	}
	if sec.MatchVersion != "" && version != sec.MatchVersion {
		return false
	}
	return true
}

// unquote 取 "..." 中的内容。
func unquote(line string) string {
	parts := strings.SplitN(line, `"`, 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// isSectionStart 判断是否为段头。
func isSectionStart(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "[")
}

// write 写回文件。
func write(path string, lines []string) error {
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
