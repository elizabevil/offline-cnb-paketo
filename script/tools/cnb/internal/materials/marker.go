package materials

import (
	"encoding/json"
	"learn-cnb-paketo/tools/cnb/internal/lock"
	"os"
	"path/filepath"

	"learn-cnb-paketo/tools/cnb/internal/timefmt"
)

// Marker 记录一次同步的来源与结果。
type Marker struct {
	ID        string        `json:"id"`
	Version   string        `json:"version"`
	Platform  lock.Platform `json:"platform"`
	SHA256    string        `json:"sha256"`
	Size      int64         `json:"size"`
	SourceURL string        `json:"source_url,omitempty"`
	// Unpack 是解包配置指纹（kind/strip/dir/expect），变化时强制重解包。
	Unpack string `json:"unpack"`
	// Extractor 保留旧字段用于兼容历史 marker。
	Extractor string   `json:"extractor,omitempty"`
	Files     []string `json:"files,omitempty"`
	SyncedAt  string   `json:"synced_at"`
}

// syncMarkerPath 返回 JSON marker 路径（与 legacy .sha256 区分）。
func syncMarkerPath(bundleDir string, platform lock.Platform, r lock.AssetRef) string {
	return filepath.Join(binariesDir(bundleDir, platform), DirMeta, r.String()+SyncSuffix)
}

// readMarker 读取 marker；不存在时返回 nil。
func readMarker(bundleDir string, platform lock.Platform, r lock.AssetRef) *Marker {
	data, err := os.ReadFile(syncMarkerPath(bundleDir, platform, r))
	if err != nil {
		return nil
	}
	var m Marker
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return &m
}

// writeMarker 写入 marker。
func writeMarker(bundleDir string, platform lock.Platform, m Marker) error {
	path := syncMarkerPath(bundleDir, platform, lock.AssetRef{ID: m.ID, Version: m.Version})
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	m.SyncedAt = timefmt.String()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// writeLegacyMarker 同步写旧版 .meta/<ref>.sha256 标记，
// 保持 materials verify（bundle 包）与 sync 的状态判断一致。
func writeLegacyMarker(bundleDir string, platform lock.Platform, ref, sha string) error {
	path := filepath.Join(binariesDir(bundleDir, platform), DirMeta, ref+".sha256")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	value := sha
	if value == "" {
		value = MarkerUnpinnedValue
	}
	return os.WriteFile(path, []byte(value), 0o644)
}

// MarkerUnpinnedValue 是未固定哈希时的占位值。
const MarkerUnpinnedValue = "unpinned"
