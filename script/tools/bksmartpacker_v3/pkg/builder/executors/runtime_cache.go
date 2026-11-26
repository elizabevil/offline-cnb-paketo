package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"bksmartpacker/pkg/log"
)

type runtimeCacheSnapshot struct {
	Images      map[string]bool
	ImagesKnown bool
	Size        int64
	SizeKnown   bool
}

func (s runtimeCacheSnapshot) count() int { return len(s.Images) }

func (e *ContainerExecutor) snapshotRuntimeCache() runtimeCacheSnapshot {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cli := newRuntimeCLI(e.runtime.Inner)
	snapshot := runtimeCacheSnapshot{Images: map[string]bool{}}
	if ids, err := cli.images(ctx); err == nil {
		snapshot.ImagesKnown = true
		for _, id := range ids {
			snapshot.Images[id] = true
		}
	}
	if size, known := cli.imageSize(ctx); known {
		snapshot.Size = size
		snapshot.SizeKnown = true
	}
	return snapshot
}

func parsePodmanImageSize(output string) (int64, bool) {
	var rows []struct {
		Type    string `json:"Type"`
		RawSize int64  `json:"RawSize"`
	}
	if err := json.Unmarshal([]byte(output), &rows); err != nil {
		return 0, false
	}
	for _, row := range rows {
		if row.Type == "Images" {
			return row.RawSize, true
		}
	}
	return 0, false
}

func classifyRuntimeCache(before, after runtimeCacheSnapshot) (state string, reused int) {
	if !before.ImagesKnown || !after.ImagesKnown {
		return "unknown", 0
	}
	if before.count() == 0 {
		return "cold", 0
	}
	for id := range before.Images {
		if after.Images[id] {
			reused++
		}
	}
	switch {
	case reused == before.count():
		return "hit", reused
	case reused > 0:
		return "partial", reused
	default:
		return "miss", 0
	}
}

func (e *ContainerExecutor) reportRuntimeCache(before runtimeCacheSnapshot) {
	after := e.snapshotRuntimeCache()
	state, reused := classifyRuntimeCache(before, after)
	ids := make([]string, 0, len(after.Images))
	for id := range after.Images {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	log.Info("runtime cache",
		"state", state,
		"cache_hit", state == "hit" || state == "partial",
		"images_known", before.ImagesKnown && after.ImagesKnown,
		"before_images", before.count(),
		"before_size", formatCacheSize(before),
		"after_images", after.count(),
		"after_size", formatCacheSize(after),
		"reused_images", reused,
		"images", ids,
	)
}

func formatCacheSize(snapshot runtimeCacheSnapshot) string {
	if !snapshot.SizeKnown {
		return "unknown"
	}
	return formatBytes(snapshot.Size)
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%dB", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
