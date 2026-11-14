package lock

import (
	"fmt"

	"learn-cnb-paketo/tools/cnb/internal/tomledit"
)

// SetSHA 回填某平台物料的 sha256。
func SetSHA(path, refStr string, platform Platform, actual string) (bool, error) {
	r, err := ParseRef(refStr)
	if err != nil {
		return false, err
	}
	return tomledit.Set(path, platformSection(r, platform), KeySHA256, actual)
}

// SetSize 回填某平台物料的大小。
func SetSize(path, refStr string, platform Platform, size int64) (bool, error) {
	r, err := ParseRef(refStr)
	if err != nil {
		return false, err
	}
	return tomledit.Set(path, platformSection(r, platform), KeySize, fmt.Sprint(size))
}

// platformSection 构造平台段的定位条件（段名由 schema.go 统一生成）。
func platformSection(r AssetRef, platform Platform) tomledit.Section {
	return tomledit.Section{
		Header:       PlatformSection(platform),
		WithinAsset:  true,
		MatchID:      r.ID,
		MatchVersion: r.Version,
	}
}
