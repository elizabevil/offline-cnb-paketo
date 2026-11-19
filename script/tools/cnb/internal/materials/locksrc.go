package materials

import (
	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/spec/manifest"
)

// lockSource 返回某平台已锁定的来源信息。
// 第二个返回值为 true 表示来自 assets.lock.toml；false 表示回落到清单。
func lockSource(a *manifest.Asset, platform lock.Platform, opts Options) (manifest.LockedSource, bool) {
	if opts.LockFile != nil {
		if la, ok := opts.LockFile.FindLocked(a.ID, a.Version); ok {
			if src, ok := la.Platform[string(platform)]; ok {
				return src, true
			}
		}
	}
	return manifest.LockedSource{}, false
}

// setLock 写入一个平台的锁定条目。
