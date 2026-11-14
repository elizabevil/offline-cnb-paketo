package bundle

import (
	"os"
	"path/filepath"
	"strings"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// 布局（与 script/lib/common.sh 一致）：
//
//	<platform>/archives/<id>/<version>/<basename(url)>
//	<platform>/binaries/<extract_to>/
//	<platform>/binaries/.meta/<id>.sha256
func archivesDir(bundleDir string, platform lock.Platform) string {
	return filepath.Join(bundleDir, string(platform), DirArchives)
}

func binariesDir(bundleDir string, platform lock.Platform) string {
	return filepath.Join(bundleDir, string(platform), DirBinaries)
}

func markerPath(bundleDir string, platform lock.Platform, ref lock.Ref) string {
	return filepath.Join(binariesDir(bundleDir, platform), DirMeta, ref+MarkerSuffix)
}

func archivePath(bundleDir string, platform lock.Platform, a *lock.Asset) string {
	url, _ := a.URL(platform)
	return filepath.Join(archivesDir(bundleDir, platform), a.ID, a.Version, filepath.Base(url))
}

// markerMatches 判断解压标记是否与 lock 中的哈希一致。
func markerMatches(bundleDir string, platform lock.Platform, a *lock.Asset) bool {
	data, err := os.ReadFile(markerPath(bundleDir, platform, a.Ref()))
	if err != nil {
		return false
	}
	recorded := string(data)
	want := a.SHA256(platform)
	recorded = strings.TrimSpace(recorded)
	if want == "" {
		return recorded == MarkerUnpinned
	}
	return recorded == want
}

// expectOK 检查声明的产物是否齐全。
func expectOK(bundleDir string, platform lock.Platform, a *lock.Asset) bool {
	return pathsExist(binariesDir(bundleDir, platform), a.ExpectedPaths())
}

// pathsExist 判断 base 下的相对路径是否全部存在。
func pathsExist(base string, rels []string) bool {
	for _, rel := range rels {
		if _, err := os.Stat(filepath.Join(base, rel)); err != nil {
			return false
		}
	}
	return true
}

// isExtracted 判断物料是否已正确解压。
func isExtracted(bundleDir string, platform lock.Platform, a *lock.Asset) bool {
	return markerMatches(bundleDir, platform, a) && expectOK(bundleDir, platform, a)
}

// fileSHA256 计算文件哈希。
func fileSHA256(path string) (string, error) { return fsutil.SHA256(path) }
