package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"learn-cnb-paketo/tools/cnb/internal/lock"
	"learn-cnb-paketo/tools/cnb/internal/materials"
	"learn-cnb-paketo/tools/cnb/internal/workspace"
)

// newSyncCmd 下载 / 解包 / 打标记。
func newSyncCmd() *cobra.Command {
	var (
		offline bool
		check   bool
		jobs    int
		proxy   string
		timeout time.Duration
		retries int
		headers []string
	)
	cmd := &cobra.Command{
		Use:   "sync [ids...]",
		Short: "download, verify and extract materials",
		RunE: func(_ *cobra.Command, ids []string) error {
			ctx, err := workspace.Open(workspace.Options{
				Repo: opts.repo, LockPath: opts.lockPath, Bundle: opts.bundle, Platform: lock.Platform(opts.platform),
			})
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			fetch := materials.FetchConfig{
				Proxy:     proxy,
				Timeout:   timeout,
				Retries:   retries,
				Headers:   parseHeaders(headers),
				UserAgent: "cnb-materials/1",
			}
			results, err := materials.Sync(ctx.Lock, ids, materials.Options{
				BundleDir: ctx.Bundle,
				LockPath:  ctx.LockPath,
				Platform:  ctx.Platform,
				Fetch:     fetch,
				Jobs:      jobs,
				Offline:   offline,
				Check:     check,
				Logger:    logr,
			})
			for _, r := range results {
				logr.Info("material", "ref", r.Ref, "action", r.Action.String())
			}
			if err != nil {
				return &exitError{code: ExitFailure, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&offline, "offline", false, "verify only, never download")
	cmd.Flags().BoolVar(&check, "check", false, "verify only")
	cmd.Flags().IntVar(&jobs, "jobs", 4, "parallel downloads")
	cmd.Flags().StringVar(&proxy, "proxy", "", "显式 HTTP 代理（默认读取 HTTPS_PROXY/HTTP_PROXY）")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "单次请求超时（默认 30m）")
	cmd.Flags().IntVar(&retries, "retries", 0, "可重试错误的最大尝试次数（默认 3）")
	cmd.Flags().StringArrayVar(&headers, "header", nil, "附加请求头，格式 K=V（可重复）")
	return cmd
}

// parseHeaders 把 K=V 列表解析成 map。
func parseHeaders(items []string) map[string]string {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]string, len(items))
	for _, item := range items {
		if k, v, ok := strings.Cut(item, "="); ok {
			out[k] = v
		}
	}
	return out
}
