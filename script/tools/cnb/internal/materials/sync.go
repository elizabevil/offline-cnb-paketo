// Package materials 负责物料同步：下载、校验、解包与标记。
package materials

import (
	"fmt"
	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"log/slog"
	"sync"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// Options 是同步参数。
type Options struct {
	BundleDir string
	Platform  lock.Platform
	Jobs      int
	Offline   bool
	Check     bool
	Logger    *slog.Logger
}

// Action 描述一次同步的动作结果。
type Action string

// 同步动作的取值。
const (
	// ActionSkipped 表示已是最新，无需处理。
	ActionSkipped Action = "skipped"
	// ActionDownloaded 表示已下载归档但未解包（保留给分段流程）。
	ActionDownloaded Action = "downloaded"
	// ActionExtracted 表示已下载并解包。
	ActionExtracted Action = "extracted"
	// ActionFailed 表示处理失败。
	ActionFailed Action = "failed"
)

// String 便于日志与输出。
func (a Action) String() string { return string(a) }

// Result 是单个物料的同步结果。
type Result struct {
	Ref    lock.Ref
	Action Action
	Err    error
}

// Sync 同步指定物料（ids 为空表示全部）。
func Sync(l *lock.File, ids []string, opts Options) ([]Result, error) {
	assets, err := l.Select(ids)
	if err != nil {
		return nil, err
	}
	if opts.Jobs <= 0 {
		opts.Jobs = 4
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	results := make([]Result, len(assets))
	sem := make(chan struct{}, opts.Jobs)
	var wg sync.WaitGroup
	for i := range assets {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			a := &assets[i]
			results[i] = syncOne(a, opts)
		}(i)
	}
	wg.Wait()

	var failed int
	for _, r := range results {
		if r.Err != nil {
			failed++
			opts.Logger.Error("sync failed", "ref", r.Ref, "err", r.Err)
		}
	}
	if failed > 0 {
		return results, fmt.Errorf("%d asset(s) failed", failed)
	}
	return results, nil
}

// syncOne 同步单个物料。
func syncOne(a *lock.Asset, opts Options) Result {
	parsed, _ := lock.ParseRef(a.Ref())
	res := Result{Ref: a.Ref(), Action: ActionSkipped}
	archive := archivePath(opts.BundleDir, opts.Platform, a)
	binDir := binariesDir(opts.BundleDir, opts.Platform)

	if m := readMarker(opts.BundleDir, opts.Platform, parsed); m != nil && m.Extractor == extractorID(a) && m.SHA256 == a.SHA256(opts.Platform) {
		return res
	}
	// 兼容旧标记（fetch.sh 的 .sha256）：产物齐全且归档校验通过时补写 marker
	if m := readMarker(opts.BundleDir, opts.Platform, parsed); m == nil && isExtracted(opts.BundleDir, opts.Platform, a) {
		if ok, _ := verifyArchive(archive, a.SHA256(opts.Platform), a.Size(opts.Platform)); ok || a.SHA256(opts.Platform) == "" {
			_ = writeLegacyMarker(opts.BundleDir, opts.Platform, a.Ref(), a.SHA256(opts.Platform))
			_ = writeMarker(opts.BundleDir, opts.Platform, Marker{
				ID: parsed.ID, Version: parsed.Version, Platform: opts.Platform, SHA256: a.SHA256(opts.Platform),
				Size: fsutil.Size(archive), Extractor: extractorID(a),
			})
			return res
		}
	}
	if opts.Check || opts.Offline {
		if !isExtracted(opts.BundleDir, opts.Platform, a) {
			res.Err = fmt.Errorf("offline: %s not synced", a.Ref())
		}
		return res
	}

	if err := ensureArchive(a, archive, opts); err != nil {
		res.Err = err
		res.Action = ActionFailed
		return res
	}
	files, err := extractAsset(a, archive, binDir)
	if err != nil {
		res.Err = err
		res.Action = ActionFailed
		return res
	}
	res.Action = ActionExtracted
	if err := writeLegacyMarker(opts.BundleDir, opts.Platform, a.Ref(), a.SHA256(opts.Platform)); err != nil {
		res.Err = err
	}

	size := fsutil.Size(archive)
	if err := writeMarker(opts.BundleDir, opts.Platform, Marker{
		ID: parsed.ID, Version: parsed.Version, Platform: opts.Platform, SHA256: a.SHA256(opts.Platform),
		Size: size, Extractor: extractorID(a), Files: files,
	}); err != nil {
		res.Err = err
	}
	return res
}

// extractorID 由解包规则生成，规则变化时 marker 自动失效。
func extractorID(a *lock.Asset) string {
	return string(a.Unpack.Kind) + "/v2"
}

// isExtracted 判断预期产物是否齐全。
