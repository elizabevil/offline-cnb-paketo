package materials

import (
	"encoding/json"
	"os"
	"path/filepath"

	"learn-cnb-paketo/tools/cnb/internal/ref"

	"learn-cnb-paketo/tools/cnb/internal/timefmt"
)

// markerSuffix 是同步标记的扩展名。
const markerSuffix = ".sync.json"

// Marker 记录一次同步的来源与结果。
type Marker struct {
	ID        string   `json:"id"`
	Version   string   `json:"version"`
	Platform  string   `json:"platform"`
	SHA256    string   `json:"sha256"`
	Size      int64    `json:"size"`
	SourceURL string   `json:"source_url,omitempty"`
	Extractor string   `json:"extractor"`
	Files     []string `json:"files,omitempty"`
	SyncedAt  string   `json:"synced_at"`
}

// markerPath 返回 marker 路径（文件名为 id@version）。
func markerPath(binDir string, r ref.Ref) string {
	return filepath.Join(binDir, ".meta", r.String()+markerSuffix)
}

// readMarker 读取 marker；不存在时返回 nil。
func readMarker(binDir string, r ref.Ref) *Marker {
	data, err := os.ReadFile(markerPath(binDir, r))
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
func writeMarker(binDir string, m Marker) error {
	path := markerPath(binDir, ref.Ref{ID: m.ID, Version: m.Version})
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
