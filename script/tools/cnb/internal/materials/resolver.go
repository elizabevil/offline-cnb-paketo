package materials

import (
	"context"
	"fmt"

	"learn-cnb-paketo/tools/cnb/internal/lock"
)

// artifact 是一次「取得可用产物」的结果。
type artifact struct {
	// SHA256 是本次实际使用的哈希（可能来自回填或既有文件）。
	SHA256 string
	// Size 是归档字节数；0 表示未知。
	Size int64
	// Pin 非空时表示需要写回 lock。
	Pin *Pin
}

// resolver 负责按来源类型取得产物。
//
// 新增来源类型（如 oci）时实现该接口，并在 resolverFor 中注册即可，
// sync 主流程不需要再按 kind 分支。
type resolver interface {
	Resolve(ctx context.Context, a *lock.Asset, opts Options) (artifact, error)
}

// resolverFor 按 Source.Kind 选择实现。
func resolverFor(a *lock.Asset, platform lock.Platform) (resolver, error) {
	switch a.Platform[string(platform)].Kind {
	case "", "http":
		return httpResolver{}, nil
	case "local":
		return localResolver{}, nil
	default:
		return nil, fmt.Errorf("%s: 不支持的来源类型 %q", a.Ref(), a.Platform[string(platform)].Kind)
	}
}

// httpResolver 下载归档（支持 mirrors 与手工预置）。
type httpResolver struct{}

func (httpResolver) Resolve(ctx context.Context, a *lock.Asset, opts Options) (artifact, error) {
	archive := archivePath(opts.BundleDir, opts.Platform, a)
	sha, size, err := ensureArchive(ctx, a, archive, opts)
	if err != nil {
		return artifact{}, err
	}
	if sha != "" {
		return artifact{SHA256: sha, Size: size, Pin: &Pin{SHA256: sha, Size: size}}, nil
	}
	return artifact{SHA256: a.SHA256(opts.Platform), Size: a.Size(opts.Platform)}, nil
}

// localResolver 由 materials publish 产出，不下载。
type localResolver struct{}

func (localResolver) Resolve(_ context.Context, a *lock.Asset, opts Options) (artifact, error) {
	if !expectOK(opts.BundleDir, opts.Platform, a) {
		return artifact{}, fmt.Errorf("%s not published; run cnb materials publish %s", a.Ref(), a.ID)
	}
	return artifact{SHA256: a.SHA256(opts.Platform), Size: a.Size(opts.Platform)}, nil
}
