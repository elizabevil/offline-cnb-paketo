package manifest

import (
	"os"
	"regexp"
	"strings"
)

// The patterns are compiled once: EditVersion is called per updated asset.
var (
	assetIDLine = regexp.MustCompile(`^\s*id\s*=\s*["']([^"']+)["']\s*$`)
	assetVer    = regexp.MustCompile(`^(\s*version\s*=\s*["'])([^"']*)(["']\s*)(.*)$`)
)

// assetBlockStart is the array-of-tables header that opens one asset.
const assetBlockStart = "[[asset]]"

// assetSubsectionPrefix marks tables that belong to the current asset.
const assetSubsectionPrefix = "[asset."

// EditVersion rewrites the version line of one asset block, keeping comments and
// formatting untouched.
//
// Blocks are delimited by [[asset]] headers; nested tables such as
// [asset.unpack] stay inside the block, other top-level tables end it.
func EditVersion(path, id, oldVersion, newVersion string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(data), "\n")

	inBlock := false
	idMatched := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == assetBlockStart:
			inBlock, idMatched = true, false
			continue
		case !inBlock:
			continue
		case isBlockEnd(trimmed):
			inBlock = false
			continue
		}
		if m := assetIDLine.FindStringSubmatch(line); m != nil {
			idMatched = m[1] == id
			continue
		}
		if !idMatched {
			continue
		}
		m := assetVer.FindStringSubmatch(line)
		if m == nil || (oldVersion != "" && m[2] != oldVersion) {
			continue
		}
		lines[i] = m[1] + newVersion + m[3] + m[4]
		return true, writeKeepingMode(path, strings.Join(lines, "\n"))
	}
	return false, &NotFoundInManifest{ID: id}
}

// isBlockEnd reports whether a table header closes the current asset block.
func isBlockEnd(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "[") {
		return false
	}
	// Nested tables of the same asset keep the block open.
	return !strings.HasPrefix(trimmed, assetSubsectionPrefix)
}

// writeKeepingMode writes data while preserving the original file mode.
func writeKeepingMode(path, content string) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(path, []byte(content), mode)
}

// NotFoundInManifest 表示清单中没有对应物料。
type NotFoundInManifest struct{ ID string }

func (e *NotFoundInManifest) Error() string { return e.ID + " not found in assets.toml" }
