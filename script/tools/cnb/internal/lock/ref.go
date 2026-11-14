package lock

import (
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/errs"
)

// AssetRef 是解析后的物料引用 id@version。
type AssetRef struct {
	ID      string
	Version string
}

// ParseRef 解析 "id@version"；没有 @ 时 Version 为空。
func ParseRef(s string) (AssetRef, error) {
	if s == "" {
		return AssetRef{}, &errs.Invalid{Field: "ref", Value: s, Reason: "empty"}
	}
	id, version, hasVer := strings.Cut(s, "@")
	if id == "" {
		return AssetRef{}, &errs.Invalid{Field: "ref", Value: s, Reason: "empty id"}
	}
	if hasVer && version == "" {
		return AssetRef{}, &errs.Invalid{Field: "ref", Value: s, Reason: "empty version"}
	}
	return AssetRef{ID: id, Version: version}, nil
}

// MustParseRef 用于已知合法的静态字符串。
func MustParseRef(s string) AssetRef {
	r, err := ParseRef(s)
	if err != nil {
		panic(err)
	}
	return r
}

// String 还原为 id@version 形式。
func (r AssetRef) String() string {
	if r.Version == "" {
		return r.ID
	}
	return r.ID + "@" + r.Version
}
