package lock

import (
	"learn-cnb-paketo/tools/cnb/internal/tomledit"
)

// SetLocalSHA 回填本地物料的 sha256（[asset.local] 段）。
func SetLocalSHA(path, refStr, actual string) (bool, error) {
	r, err := ParseRef(refStr)
	if err != nil {
		return false, err
	}
	return tomledit.Set(path, tomledit.Section{
		Header:       LocalSection(),
		WithinAsset:  true,
		MatchID:      r.ID,
		MatchVersion: r.Version,
	}, KeySHA256, actual)
}
