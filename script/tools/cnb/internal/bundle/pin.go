package bundle

import (
	"fmt"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// Suggestions 反推本地归档的 sha256（不写回 lock）。
func Suggestions(l *lock.File, bundleDir string, platform lock.Platform, ids []string) ([]Suggestion, error) {
	if !lock.KnownPlatforms[platform] {
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
	assets, err := l.Select(ids)
	if err != nil {
		return nil, err
	}

	out := make([]Suggestion, 0, len(assets))
	for i := range assets {
		a := &assets[i]
		archive := archivePath(bundleDir, platform, a)
		sum, err := fileSHA256(archive)
		if err != nil {
			continue // 归档缺失时跳过，由调用方统一提示
		}
		out = append(out, Suggestion{Ref: a.Ref(), Platform: platform, SHA256: sum})
	}
	return out, nil
}
