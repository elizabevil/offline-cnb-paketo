// Package materials 负责物料同步：下载、校验、解包与标记。
package materials

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"learn-cnb-paketo/tools/cnb/internal/fsutil"
	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// Options 是同步参数。
type Options struct {
	BundleDir string
	// Fetch 控制下载（超时、重试、代理、请求头）。
	Fetch FetchConfig
	// LockPath 非空且允许写回时，未固定的 sha256/size 会在下载后自动回填。
	LockPath string
	Platform lock.Platform
	Jobs     int
	Offline  bool
	Check    bool
	Logger   *slog.Logger
}

// Pin 是待回填的 sha256 与大小。
type Pin struct {
	SHA256 string
	Size   int64
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
	// Pinned 非空时表示本次下载需要回填 lock（由 Sync 统一写回）。
	Pinned *Pin
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

	if err := writePins(results, opts); err != nil {
		return results, err
	}

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

	resolver, err := resolverFor(a, opts.Platform)
	if err != nil {
		res.Action = ActionFailed
		res.Err = err
		return res
	}
	local := a.IsLocal(opts.Platform)
	if local {
		if _, err := resolver.Resolve(context.Background(), a, opts); err != nil {
			res.Action = ActionFailed
			res.Err = err
		}
		return res
	}

	archive := archivePath(opts.BundleDir, opts.Platform, a)
	binDir := binariesDir(opts.BundleDir, opts.Platform)
	unpack := unpackID(a)
	pinned := a.SHA256(opts.Platform)
	marker := readMarker(opts.BundleDir, opts.Platform, parsed)

	// 快速路径：已固定 + marker 匹配 + 产物齐全 + 归档仍通过校验
	if pinned != "" && marker != nil && marker.SHA256 == pinned && marker.Unpack == unpack &&
		expectOK(opts.BundleDir, opts.Platform, a) {
		if ok, _ := verifyArchive(archive, pinned, a.Size(opts.Platform)); ok {
			return res
		}
	}

	if opts.Check || opts.Offline {
		if !isExtracted(opts.BundleDir, opts.Platform, a) {
			res.Err = fmt.Errorf("offline: %s not synced", a.Ref())
		}
		return res
	}

	art, err := resolver.Resolve(context.Background(), a, opts)
	if err != nil {
		res.Err = err
		res.Action = ActionFailed
		return res
	}
	res.Pinned = art.Pin
	effectiveSHA := art.SHA256

	// 归档未变化且解包配置一致、产物齐全：只需补写标记
	if marker != nil && marker.SHA256 == effectiveSHA && marker.Unpack == unpack &&
		expectOK(opts.BundleDir, opts.Platform, a) {
		res.Action = ActionSkipped
		return res
	}

	// 重新解包前先清掉旧目录，避免新旧文件混合
	if err := cleanTarget(a, binDir); err != nil {
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
	if err := writeLegacyMarker(opts.BundleDir, opts.Platform, a.Ref(), effectiveSHA); err != nil {
		res.Err = err
	}

	size := fsutil.Size(archive)
	if art.Size > 0 {
		size = art.Size
	}
	if err := writeMarker(opts.BundleDir, opts.Platform, Marker{
		ID: parsed.ID, Version: parsed.Version, Platform: opts.Platform, SHA256: effectiveSHA,
		Size: size, Unpack: unpack, Files: files,
	}); err != nil {
		res.Err = err
	}
	return res
}

// unpackID 由解包配置生成指纹，配置变化时 marker 失效。
func unpackID(a *lock.Asset) string {
	raw := fmt.Sprintf("%s|%d|%s|%v|%v", a.Unpack.Kind, a.Unpack.Strip, a.Unpack.Dir, a.Unpack.Expect, a.Unpack.Include)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// cleanTarget 删除该物料的解包目标。
func cleanTarget(a *lock.Asset, binDir string) error {
	if a.Unpack.Kind == lock.KindBin {
		rel := a.ExpectedPaths()[0]
		return os.Remove(filepath.Join(binDir, rel))
	}
	return os.RemoveAll(filepath.Join(binDir, a.Dir()))
}

// writePins 串行回填所有未固定的 sha256/size（避免并发读改写丢更新）。
func writePins(results []Result, opts Options) error {
	if opts.Check || opts.Offline || opts.LockPath == "" {
		return nil
	}
	for _, r := range results {
		if r.Pinned == nil {
			continue
		}
		if _, err := lock.SetSHA(opts.LockPath, r.Ref, opts.Platform, r.Pinned.SHA256); err != nil {
			return err
		}
		if _, err := lock.SetSize(opts.LockPath, r.Ref, opts.Platform, r.Pinned.Size); err != nil {
			return err
		}
		opts.Logger.Info("pinned", "ref", r.Ref, "platform", opts.Platform, "sha256", r.Pinned.SHA256)
	}
	return nil
}

// isExtracted 判断预期产物是否齐全。
