package materials

import (
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// Layout (kept in sync with the bundle produced by script/lib/common.sh):
//
//	<platform>/archives/<id>/<version>/<basename(url)>
//	<platform>/binaries/<extract_to>/
//	<platform>/binaries/.meta/<id>@<version>.sha256
func archivesDir(bundleDir string, platform lock.Platform) string {
	return filepath.Join(bundleDir, string(platform), DirArchives)
}

func binariesDir(bundleDir string, platform lock.Platform) string {
	return filepath.Join(bundleDir, string(platform), DirBinaries)
}

func markerPath(bundleDir string, platform lock.Platform, ref lock.AssetRef) string {
	return filepath.Join(binariesDir(bundleDir, platform), DirMeta, ref.String()+MarkerSuffix)
}

func archivePath(bundleDir string, platform lock.Platform, a *lock.Asset) string {
	url, _ := a.URL(platform)
	return filepath.Join(archivesDir(bundleDir, platform), a.ID, a.Version, filepath.Base(url))
}

// markerMatches reports whether the unpack marker matches the locked hash.
func markerMatches(bundleDir string, platform lock.Platform, a *lock.Asset, want string) bool {
	data, err := os.ReadFile(markerPath(bundleDir, platform, lock.MustParseRef(a.Ref())))
	if err != nil {
		return false
	}
	recorded := strings.TrimSpace(string(data))
	if want == "" {
		return recorded == MarkerUnpinned
	}
	return recorded == want
}

// expectOK reports whether every declared artifact exists.
func expectOK(bundleDir string, platform lock.Platform, a *lock.Asset) bool {
	base := binariesDir(bundleDir, platform)
	for _, rel := range a.ExpectedPaths(platform) {
		if !fsutil.Exists(filepath.Join(base, rel)) {
			return false
		}
	}
	return true
}

// isExtracted reports whether the material is unpacked and complete.
func isExtracted(bundleDir string, platform lock.Platform, a *lock.Asset, want string) bool {
	return markerMatches(bundleDir, platform, a, want) && expectOK(bundleDir, platform, a)
}
